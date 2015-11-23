package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

// Writer implements a Writer and writes to the Datadog API bucketed stats & spans
type Writer struct {
	endpoint        BucketEndpoint          // config, where we're writing the data
	in              chan model.AgentPayload // data input, payloads of concentrated spans/stats
	payloadsToWrite []model.AgentPayload    // buffers to write to the API and currently written to from upstream
	mu              sync.Mutex              // mutex on data above

	Worker
}

// NewWriter returns a new Writer
func NewWriter(conf *config.AgentConfig) *Writer {
	var endpoint BucketEndpoint
	if conf.APIEnabled {
		endpoint = NewAPIEndpoint(conf.APIEndpoint, conf.APIKey)
	} else {
		log.Info("using null endpoint")
		endpoint = NullEndpoint{}
	}

	w := Writer{
		endpoint: endpoint,
		in:       make(chan model.AgentPayload),
	}
	w.Init()

	return &w
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (w *Writer) Start() {
	w.wg.Add(1)
	go w.run()

	log.Info("Writer started")
}

// We rely on the concentrator ticker to flush periodically traces "aligning" on the buckets
// (it's not perfect, but we don't really care, traces of this stats bucket may arrive in the next flush)
func (w *Writer) run() {
	for {
		select {
		case p := <-w.in:
			log.Info("Received a payload, initiating a flush")
			w.mu.Lock()
			w.payloadsToWrite = append(w.payloadsToWrite, p)
			w.mu.Unlock()
			w.Flush()
		case <-w.exit:
			log.Info("Writer exiting, trying to flush all remaining data")
			w.Flush()
			w.wg.Done()
			return
		}
	}
}

// Flush actually writes the data in the API
func (w *Writer) Flush() {
	// TODO[leo], do we want this to be async?
	w.mu.Lock()
	defer w.mu.Unlock()

	// number of successfully flushed buckets
	flushed := 0
	total := len(w.payloadsToWrite)
	if total == 0 {
		return
	}

	// FIXME: this is not ideal we might want to batch this into a single http call
	for _, p := range w.payloadsToWrite {

		err := w.endpoint.Write(p)
		if err != nil {
			log.Errorf("Error writing bucket: %s", err)
			break
		}
		flushed++
	}

	if flushed == total {
		// all payloads were properly flushed.
		w.payloadsToWrite = nil
	} else if 0 < flushed {
		w.payloadsToWrite = w.payloadsToWrite[flushed:]
	}

	log.Infof("Flushed %d/%d payloads", flushed, total)
}

// BucketEndpoint is a place where we can write payloads.
type BucketEndpoint interface {
	Write(b model.AgentPayload) error
}

// APIEndpoint is the api we write to.
type APIEndpoint struct {
	hostname     string
	apiKey       string
	url          string
	collectorURL string
}

// NewAPIEndpoint creates an endpoint writing to the given url and apiKey.
func NewAPIEndpoint(url string, apiKey string) APIEndpoint {
	// FIXME[leo]: allow overriding it from config?
	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("Could not get hostname: %v", err))
	}

	collectorURL := url + "/collector"
	return APIEndpoint{hostname: hostname, apiKey: apiKey, url: url, collectorURL: collectorURL}
}

// Write writes the bucket to the api.
func (a APIEndpoint) Write(payload model.AgentPayload) error {
	startFlush := time.Now()
	payload.HostName = a.hostname

	jsonStr, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", a.collectorURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}

	queryParams := req.URL.Query()
	queryParams.Add("api_key", a.apiKey)
	req.URL.RawQuery = queryParams.Encode()
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	flushTime := time.Since(startFlush)
	log.Infof("Payload flushed to the API (time=%s, size=%d)", flushTime, len(jsonStr))
	Statsd.Gauge("trace_agent.writer.flush_duration", flushTime.Seconds(), nil, 1)
	Statsd.Count("trace_agent.writer.payload_bytes", int64(len(jsonStr)), nil, 1)

	return nil
}

// NullEndpoint is a place where bucket go to die.
type NullEndpoint struct{}

// Write drops the bucket on the floor.
func (ne NullEndpoint) Write(p model.AgentPayload) error {
	log.Debug("Null endpoint is dropping bucket")
	return nil
}

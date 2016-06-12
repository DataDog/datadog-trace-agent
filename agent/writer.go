package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/statsd"
)

// Writer implements a Writer and writes to the Datadog API bucketed stats & spans
type Writer struct {
	endpoint BucketEndpoint // config, where we're writing the data

	in         chan model.AgentPayload     // data input, payloads of concentrated spans/stats
	inServices chan model.ServicesMetadata // the metadata we receive form the client to be stored in the backend

	mu              sync.Mutex             // mutex on data above
	payloadsToWrite []model.AgentPayload   // buffers to write to the API and currently written to from upstream
	svcs            model.ServicesMetadata // the current up-to-date services
	svcsVer         int64                  // the current version of services
	svcsFlushed     int64                  // the last flushed version of services

	exit chan struct{}
}

// NewWriter returns a new Writer
func NewWriter(conf *config.AgentConfig, inServices chan model.ServicesMetadata) *Writer {
	var endpoint BucketEndpoint
	if conf.APIEnabled {
		endpoint = NewAPIEndpoint(conf.APIEndpoint, conf.APIKey, conf.HostName)
	} else {
		log.Info("API interface is disabled, use NullEndpoint instead")
		endpoint = NullEndpoint{}
	}

	return &Writer{
		endpoint:   endpoint,
		in:         make(chan model.AgentPayload),
		inServices: inServices,
		svcs:       make(model.ServicesMetadata),
		exit:       make(chan struct{}),
	}
}

func (w *Writer) Run() {
	for {
		select {
		case p := <-w.in:
			log.Debug("new payload received, triggering flush")
			w.mu.Lock()
			w.payloadsToWrite = append(w.payloadsToWrite, p)
			w.mu.Unlock()
			w.Flush()
			w.FlushServices()
		case sm := <-w.inServices:
			w.mu.Lock()
			w.svcs.Update(sm)
			w.svcsVer++
			w.mu.Unlock()
		case <-w.exit:
			log.Info("trying to flush all remaining data")
			w.Flush()
			return
		}
	}
}

// FlushServices initiate a flush of the services to the services endpoint
func (w *Writer) FlushServices() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.svcsFlushed == w.svcsVer {
		return
	}

	err := w.endpoint.WriteServices(w.svcs)
	if err != nil {
		log.Errorf("could not flush services: %v", err)
	}

	w.svcsFlushed = w.svcsVer
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
			log.Errorf("could not flush payload: %s", err)
			break
		}
		flushed++
	}

	// regardless if the http post was was success or not. We don't want to buffer
	//  data in case of api outage
	w.payloadsToWrite = nil

	if flushed != total {
		log.Infof("successfully flushed %d payloads to the API but remains %d to flush", flushed, total-flushed)
	}
}

// BucketEndpoint is a place where we can write payloads
type BucketEndpoint interface {
	Write(b model.AgentPayload) error
	WriteServices(s model.ServicesMetadata) error
}

// APIEndpoint is the api we write to.
type APIEndpoint struct {
	hostname string
	apiKey   string
	url      string
}

// NewAPIEndpoint creates an endpoint writing to the given url and apiKey
func NewAPIEndpoint(url string, apiKey string, hostname string) APIEndpoint {
	return APIEndpoint{hostname: hostname, apiKey: apiKey, url: url}
}

// Write writes the bucket to the API collector endpoint
func (a APIEndpoint) Write(payload model.AgentPayload) error {
	startFlush := time.Now()
	payload.HostName = a.hostname

	var body bytes.Buffer
	gz := gzip.NewWriter(&body)
	err := json.NewEncoder(gz).Encode(payload)
	if err != nil {
		return err
	}
	gz.Close()
	payloadLen := body.Len()

	req, err := http.NewRequest("POST", a.url+"/collector", &body)
	if err != nil {
		return err
	}

	queryParams := req.URL.Query()
	queryParams.Add("api_key", a.apiKey)
	req.URL.RawQuery = queryParams.Encode()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("backend responded with %d %s", resp.StatusCode, resp.Status)
	}

	flushTime := time.Since(startFlush)
	log.Infof("flushed payload to the API, time:%s, size:%d", flushTime, payloadLen)
	statsd.Client.Gauge("trace_agent.writer.flush_duration", flushTime.Seconds(), nil, 1)
	statsd.Client.Count("trace_agent.writer.payload_bytes", int64(body.Len()), nil, 1)

	return nil
}

// WriteServices writes services to the services endpoint
func (a APIEndpoint) WriteServices(s model.ServicesMetadata) error {
	jsonStr, err := json.Marshal(s)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", a.url+"/services", bytes.NewBuffer(jsonStr))
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

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("backend responded with %d %s", resp.StatusCode, resp.Status)
	}

	log.Infof("flushed %d services to the API", len(s))

	return nil
}

// NullEndpoint is a place where bucket go the void
type NullEndpoint struct{}

// Write drops the bucket on the floor
func (ne NullEndpoint) Write(p model.AgentPayload) error {
	log.Debug("Null endpoint is dropping bucket")
	return nil
}

// WriteServices NOOP
func (ne NullEndpoint) WriteServices(s model.ServicesMetadata) error {
	log.Debug("Null endpoint dropping services info: %v", s)
	return nil
}

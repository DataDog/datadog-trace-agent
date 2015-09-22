package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	log "github.com/cihub/seelog"
)

// Writer implements a Writer and writes to the Datadog API bucketed stats & spans
type Writer struct {
	endpoint       BucketEndpoint
	inBuckets      chan ConcentratorBucket // data input, buckets of concentrated spans/stats
	bucketsToWrite []ConcentratorBucket    // buffers to write to the API and currently written to from upstream
	mu             sync.Mutex              // mutex on data above

	// exit channels
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// NewWriter returns a new Writer
func NewWriter(ep BucketEndpoint, in chan ConcentratorBucket, exit chan struct{}, exitGroup *sync.WaitGroup) *Writer {
	w := Writer{
		endpoint:  ep,
		inBuckets: in,
		exit:      exit,
		exitGroup: exitGroup,
	}

	return &w
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (w *Writer) Start() {
	w.exitGroup.Add(1)
	go w.run()

	log.Info("Writer started")
}

// We rely on the concentrator ticker to flush periodically traces "aligning" on the buckets
// (it's not perfect, but we don't really care, traces of this stats bucket may arrive in the next flush)
func (w *Writer) run() {
	for {
		select {
		case bucket := <-w.inBuckets:
			log.Info("Received a bucket from concentrator, initiating a flush")
			w.mu.Lock()
			w.bucketsToWrite = append(w.bucketsToWrite, bucket)
			w.mu.Unlock()
			w.Flush()
		case <-w.exit:
			log.Info("Writer exiting, trying to flush all remaining data")
			w.Flush()
			w.exitGroup.Done()
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
	total := len(w.bucketsToWrite)
	if total == 0 {
		return
	}

	// FIXME: this is not ideal we might want to batch this into a single http call
	for _, b := range w.bucketsToWrite {

		// decide to not flush if no spans & no stats
		if b.isEmpty() {
			log.Debugf("Bucket %d sampler & stats are empty", b.Stats.Start)
			flushed++
			continue
		}

		err := w.endpoint.Write(b)
		if err != nil {
			log.Errorf("Error writing bucket: %s", err)
			break
		}
		flushed++
	}

	if flushed == total {
		// all buckets were properly flushed.
		w.bucketsToWrite = nil
	} else if 0 < flushed {
		w.bucketsToWrite = w.bucketsToWrite[flushed:]
	}

	log.Infof("Flushed %d/%d buckets", flushed, total)
}

// BucketEndpoint is a place where we can write buckets.
type BucketEndpoint interface {
	Write(b ConcentratorBucket) error
}

// APIEndpoint is the api we write to.
type APIEndpoint struct {
	url          string
	collectorURL string
}

// NewAPIEndpoint creates an endpoint writing to the given url.
func NewAPIEndpoint(url string) APIEndpoint {
	collectorURL := url + "/collector"
	return APIEndpoint{url: url, collectorURL: collectorURL}
}

// Write writes the bucket to the api.
func (a APIEndpoint) Write(b ConcentratorBucket) error {
	startFlush := time.Now()
	payload := b.buildPayload()
	log.Infof("Bucket %d being flushed to the API (%d spans)", b.Stats.Start, len(payload.Spans))

	jsonStr, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", a.collectorURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	flushTime := time.Since(startFlush)
	log.Infof("Bucket %d, flushed to the API (time=%s, size=%d)", b.Stats.Start, flushTime, len(jsonStr))
	Statsd.Gauge("trace_agent.writer.flush_duration", flushTime.Seconds(), nil, 1)
	Statsd.Count("trace_agent.writer.payload_bytes", int64(len(jsonStr)), nil, 1)

	return nil
}

// NullEndpoint is a place where bucket go to die.
type NullEndpoint struct{}

// Write drops the bucket on the floor.
func (ne NullEndpoint) Write(b ConcentratorBucket) error {
	log.Debug("Null endpoint is dropping bucket")
	return nil
}

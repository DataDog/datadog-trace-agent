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
	endpoint       string
	inBuckets      chan ConcentratorBucket // data input, buckets of concentrated spans/stats
	bucketsToWrite []ConcentratorBucket    // buffers to write to the API and currently written to from upstream
	mu             sync.Mutex              // mutex on data above

	// exit channels
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// NewWriter returns a new Writer
func NewWriter(endp string, inBuckets chan ConcentratorBucket, exit chan struct{}, exitGroup *sync.WaitGroup) *Writer {
	w := Writer{
		endpoint:  endp,
		inBuckets: inBuckets,
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

	// FIXME: this is not ideal we might want to batch this into a single http call
	for _, b := range w.bucketsToWrite {
		startFlush := time.Now()

		// decide to not flush if no spans & no stats
		if b.isEmpty() {
			log.Debugf("Bucket %d sampler & stats are empty", b.Stats.Start)
			flushed++
			continue
		}

		payload := b.getPayload()
		log.Infof("Bucket %d being flushed to the API (%d spans)", b.Stats.Start, len(payload.Spans))

		url := w.endpoint + "/collector"

		jsonStr, err := json.Marshal(payload)
		if err != nil {
			log.Errorf("Error marshalling: %s", err)
			break
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
		if err != nil {
			log.Errorf("Error creating request: %s", err)
			break
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("Error posting request: %s", err)
			break
		}
		defer resp.Body.Close()

		flushTime := time.Since(startFlush)
		log.Infof("Bucket %d, flushed to the API (time=%s, size=%d)", b.Stats.Start, flushTime, len(jsonStr))
		Statsd.Gauge("trace_agent.writer.flush_duration", flushTime.Seconds(), nil, 1)
		Statsd.Count("trace_agent.writer.payload_bytes", int64(len(jsonStr)), nil, 1)
		flushed++
	}

	if flushed != 0 {
		w.bucketsToWrite = w.bucketsToWrite[flushed:]
	} else {
		log.Warnf("Could not flush, still %d payloads to be flushed", len(w.bucketsToWrite))
	}
}

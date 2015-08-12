package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// SpanWriter implements a Writer and writes to the Datadog API spans
type SpanWriter struct {
	in         chan model.Span
	spanBuffer []model.Span
	endpoint   string

	// exit channels
	exit      chan bool
	exitGroup *sync.WaitGroup
}

// NewSpanWriter returns a new Writer
func NewSpanWriter(endp string, exit chan bool, exitGroup *sync.WaitGroup) *SpanWriter {
	return &SpanWriter{
		endpoint:  endp,
		exit:      exit,
		exitGroup: exitGroup,
	}
}

// Init initalizes the span buffer and the input channel of spans
func (w *SpanWriter) Init(in chan model.Span) {
	w.in = in

	// NOTE: should this be unbounded?
	w.spanBuffer = []model.Span{}
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (w *SpanWriter) Start() {
	// will shutdown as the input channel is closed
	go func() {
		for s := range w.in {
			w.spanBuffer = append(w.spanBuffer, s)
		}
	}()

	go w.periodicFlush()

	log.Info("SpanWriter started")
}

func (w *SpanWriter) periodicFlush() {
	ticker := time.Tick(3 * time.Second)
	for {
		select {
		case <-ticker:
			w.Flush()
		case <-w.exit:
			log.Info("SpanWriter asked to exit. Flushing and exiting")
			// FIXME, make sure w.in is closed before to make sure we received all spans
			w.Flush()
			return
		}
	}
}

// Flush the span buffer by writing to the API its contents
func (w *SpanWriter) Flush() {
	spans := w.spanBuffer
	if len(spans) == 0 {
		log.Info("Nothing to flush")
		return
	}
	w.spanBuffer = []model.Span{}
	log.Infof("SpanWriter flush to the API, %d spans", len(spans))

	payload := model.SpanPayload{
		// FIXME, this should go in a config file
		APIKey: "424242",
		Spans:  spans,
	}

	url := w.endpoint + "/spans"

	jsonStr, err := json.Marshal(payload)
	if err != nil {
		log.Errorf("Error marshalling: %s", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		log.Errorf("Error creating request: %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Error posting request: %s", err)
		return
	}
	defer resp.Body.Close()
}

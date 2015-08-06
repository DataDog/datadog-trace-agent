package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// Writer is an interface of a routine consuming and persisting spans
// somewhere
type Writer interface {
	Init(chan model.Span)
	Start()
}

// CollectorPayload is the payload sent to the API of mothership
type CollectorPayload struct {
	APIKey string       `json:"api_key"`
	Spans  []model.Span `json:"spans"`
}

// APIWriter implements a Writer and writes to the Datadog API spans
type APIWriter struct {
	in         chan model.Span
	spanBuffer []model.Span
}

// NewAPIWriter returns a new Writer
func NewAPIWriter() *APIWriter {
	return &APIWriter{}
}

// Init initalizes the span buffer and the input channel of spans
func (w *APIWriter) Init(in chan model.Span) {
	w.in = in
	w.spanBuffer = []model.Span{}
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (w *APIWriter) Start() {
	go func() {
		for s := range w.in {
			w.spanBuffer = append(w.spanBuffer, s)
		}
	}()

	go w.periodicFlush()

	log.Info("APIWriter started")
}

func (w *APIWriter) periodicFlush() {
	c := time.NewTicker(3 * time.Second).C
	for _ = range c {
		w.Flush()
	}
}

// Flush the span buffer by writing to the API its contents
func (w *APIWriter) Flush() {
	spans := w.spanBuffer
	if len(spans) == 0 {
		log.Info("Nothing to flush")
		return
	}
	w.spanBuffer = []model.Span{}
	log.Infof("Flush collector to the API, %d spans", len(spans))

	payload := CollectorPayload{
		APIKey: "424242",
		Spans:  spans,
	}

	url := "http://localhost:8012/api/v0.1/collector"

	jsonStr, err := json.Marshal(payload)
	if err != nil {
		log.Errorf("Error marshalling: %s", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		log.Errorf("error creating request: %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("error posting request: %s", err)
		return
	}
	defer resp.Body.Close()
}

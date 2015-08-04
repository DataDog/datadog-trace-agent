package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	log "github.com/cihub/seelog"

	_ "github.com/mattn/go-sqlite3"
)

type Writer interface {
	Init(chan Span)
	Start()
}

type CollectorPayload struct {
	ApiKey string `json:"api_key"`
	Spans  []Span `json:"spans"`
}

type APIWriter struct {
	in         chan Span
	spanBuffer []Span
}

func NewAPIWriter() *APIWriter {
	return &APIWriter{}
}

func (w *APIWriter) Init(in chan Span) {
	w.in = in
	w.spanBuffer = []Span{}
}

func (w *APIWriter) Start() {
	go func() {
		for s := range w.in {
			w.spanBuffer = append(w.spanBuffer, s)
		}
	}()

	go w.PeriodicFlush()

	log.Info("APIWriter started")
}

func (w *APIWriter) PeriodicFlush() {
	c := time.NewTicker(3 * time.Second).C
	for _ = range c {
		w.Flush()
	}
}

func (w *APIWriter) Flush() {
	spans := w.spanBuffer
	if len(spans) == 0 {
		log.Info("Nothing to flush")
		return
	}
	w.spanBuffer = []Span{}
	log.Infof("Flush collector to the API, %d spans", len(spans))

	payload := CollectorPayload{
		ApiKey: "424242",
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

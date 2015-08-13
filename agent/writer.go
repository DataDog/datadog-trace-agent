package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// Writer implements a Writer and writes to the Datadog API spans
type Writer struct {
	inSpan     chan model.Span
	spanBuffer []model.Span
	endpoint   string

	inStats chan model.StatsBucket

	// exit channels
	exit      chan bool
	exitGroup *sync.WaitGroup
}

// NewWriter returns a new Writer
func NewWriter(endp string, exit chan bool, exitGroup *sync.WaitGroup) *Writer {
	return &Writer{
		endpoint:  endp,
		exit:      exit,
		exitGroup: exitGroup,
	}
}

// Init initalizes the span buffer and the input channel of spans
func (w *Writer) Init(inSpan chan model.Span, inStats chan model.StatsBucket) {
	w.inSpan = inSpan
	w.inStats = inStats

	// NOTE: should this be unbounded?
	w.spanBuffer = []model.Span{}
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (w *Writer) Start() {
	// will shutdown as the input channel is closed
	go func() {
		for s := range w.inSpan {
			log.Debugf("Received a span, TID=%d, SID=%d, service=%s, resource=%s", s.TraceID, s.SpanID, s.Service, s.Resource)
			w.spanBuffer = append(w.spanBuffer, s)
		}
	}()

	go w.flushStatsBucket()

	log.Info("Writer started")
}

// We rely on the concentrator ticker to flush periodically traces "aligning" on the buckets (it's not perfect, but we don't really care, traces of this stats bucket may arrive in the next flush)
func (w *Writer) flushStatsBucket() {
	for {
		select {
		case bucket := <-w.inStats:
			log.Info("Received a stats bucket, flushing stats & bufferend spans")
			w.Flush(&bucket)
		case <-w.exit:
			log.Info("Writer asked to exit. Flushing and exiting")
			// FIXME, make sure w.inSpan is closed before to make sure we received all spans
			w.Flush(nil)
			return
		}
	}
}

// Flush the span buffer by writing to the API its contents
// FIXME: if we fail here, we must buffer the statsbucket to be able to replay them...
func (w *Writer) Flush(b *model.StatsBucket) {
	spans := w.spanBuffer

	w.spanBuffer = []model.Span{}
	log.Infof("Writer flush to the API, %d spans", len(spans))

	payload := model.SpanPayload{
		// FIXME, this should go in a config file
		APIKey: "424242",
		Spans:  spans,
	}
	if b != nil {
		payload.Stats = b
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

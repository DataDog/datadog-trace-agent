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
	endpoint string

	// All writen structs are buffered, in case sh** happens during transmissions
	inSpan      chan model.Span
	spanBuffer  []model.Span
	inStats     chan model.StatsBucket
	statsBuffer []model.StatsBucket

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
	w.statsBuffer = []model.StatsBucket{}
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
			w.statsBuffer = append(w.statsBuffer, bucket)
			w.Flush()
		case <-w.exit:
			log.Info("Writer asked to exit. Flushing and exiting")
			// FIXME, make sure w.inSpan is closed before to make sure we received all spans
			w.Flush()
			return
		}
	}
}

// Flush the span buffer by writing to the API its contents
func (w *Writer) Flush() {
	spans := w.spanBuffer
	stats := w.statsBuffer

	w.spanBuffer = []model.Span{}
	w.statsBuffer = []model.StatsBucket{}
	log.Infof("Writer flush to the API, %d spans, %d stats buckets", len(spans), len(stats))

	payload := model.SpanPayload{
		// FIXME, this should go in a config file
		APIKey: "424242",
		Spans:  spans,
		Stats:  stats,
	}

	url := w.endpoint + "/collector"

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

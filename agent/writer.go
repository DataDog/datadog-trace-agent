package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

type WriterBuffer struct {
	Spans []model.Span
	Stats *model.StatsBucket
}

// Writer implements a Writer and writes to the Datadog API spans
type Writer struct {
	endpoint string

	// All writen structs are buffered, in case sh** happens during transmissions
	inSpan  chan model.Span
	inStats chan model.StatsBucket

	toWrite []WriterBuffer
	bufLock sync.Mutex

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

	w.addNewBuffer()
}

func (w *Writer) addNewBuffer() {
	// Add a new buffer
	// FIXME: Should these buffers be unbounded?
	wb := WriterBuffer{}
	w.bufLock.Lock()
	w.toWrite = append(w.toWrite, wb)
	w.bufLock.Unlock()
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (w *Writer) Start() {
	// will shutdown as the input channel is closed
	go func() {
		for s := range w.inSpan {
			log.Debugf("Received a span, TID=%d, SID=%d, service=%s, resource=%s", s.TraceID, s.SpanID, s.Service, s.Resource)
			// Always write to last element of span
			// FIXME: mutex too slow?
			w.bufLock.Lock()
			w.toWrite[len(w.toWrite)-1].Spans = append(w.toWrite[len(w.toWrite)-1].Spans, s)
			w.bufLock.Unlock()
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
			// closing this buffer
			w.bufLock.Lock()
			w.toWrite[len(w.toWrite)-1].Stats = &bucket
			w.bufLock.Unlock()
			w.addNewBuffer()
			w.Flush()
		case <-w.exit:
			log.Info("Writer exiting")
			// FIXME? don't flush the traces we received because we didn't get the stats associated
			// w.addNewBuffer()
			// w.Flush()
			return
		}
	}
}

// Flush the span buffer by writing to the API its contents
func (w *Writer) Flush() {
	maxBuf := len(w.toWrite) - 1
	flushed := 0
	for i := 0; i < maxBuf; i++ {
		log.Infof("Writer flush to the API, %d spans", len(w.toWrite[i].Spans))

		payload := model.SpanPayload{
			// FIXME, this should go in a config file
			APIKey: "424242",
			Spans:  w.toWrite[i].Spans,
			Stats:  w.toWrite[i].Stats,
		}

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
		// if it succeeded remove from the slice
		flushed++
	}

	if flushed != 0 {
		w.bufLock.Lock()
		w.toWrite = w.toWrite[flushed:]
		log.Infof("Flushed successfully %d payloads", flushed)
		w.bufLock.Unlock()
	} else {
		log.Warnf("Could not flush, still %d payloads to be flushed", maxBuf)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// WriterBuffer contains Spans and Stats to write to the API
type WriterBuffer struct {
	Sampler Sampler
	Stats   model.StatsBucket
	// Spans   []model.Span
}

// Writer implements a Writer and writes to the Datadog API spans
type Writer struct {
	endpoint string

	// All written structs are buffered, in case sh** happens during transmissions
	inSpans chan model.Span
	inStats chan model.StatsBucket

	// Sampler configuration
	// TODO: move the sampler into a real Agent worker?
	quantiles []float64

	lastFlush int64          // last successful flush, for logging/monitoring purposes
	toWrite   []WriterBuffer // buffers to write to the API and currently written to from upstream
	bufLock   sync.Mutex     // mutex on data above

	// exit channels
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// NewWriter returns a new Writer
func NewWriter(endp string, quantiles []float64, inSpans chan model.Span, inStats chan model.StatsBucket, exit chan struct{}, exitGroup *sync.WaitGroup) *Writer {
	w := Writer{
		endpoint:  endp,
		inSpans:   inSpans,
		inStats:   inStats,
		exit:      exit,
		exitGroup: exitGroup,
		quantiles: quantiles,
	}
	w.addNewBuffer()

	return &w
}

func (w *Writer) addNewBuffer() {
	// Add a new buffer
	// FIXME: Should these buffers be unbounded?
	wb := WriterBuffer{Sampler: NewSampler()}
	w.toWrite = append(w.toWrite, wb)
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (w *Writer) Start() {
	w.exitGroup.Add(1)

	// will shutdown as the input channel is closed
	go func() {
		for s := range w.inSpans {
			// Always write to last element of span
			w.bufLock.Lock()
			w.toWrite[len(w.toWrite)-1].Sampler.AddSpan(s)
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
			log.Info("Received a stats bucket, flushing stats & buffered spans")
			// closing this buffer
			w.bufLock.Lock()
			w.toWrite[len(w.toWrite)-1].Stats = bucket
			w.addNewBuffer()
			w.bufLock.Unlock()

			w.Flush()
		case <-w.exit:
			log.Info("Writer exiting")
			// FIXME? don't flush the traces we received because we didn't get the stats associated
			// w.addNewBuffer()
			// w.Flush()
			w.exitGroup.Done()
			return
		}
	}
}

// Flush the span buffer by writing to the API its contents
func (w *Writer) Flush() {
	// Do not flush the buffer we just added
	maxBuf := len(w.toWrite) - 1
	flushed := 0

	// FIXME: this is not ideal we might want to batch this into a single http call
	for i := 0; i < maxBuf; i++ {
		// decide to not flush if no spans & no stats
		if w.toWrite[i].Sampler.IsEmpty() && len(w.toWrite[i].Stats.Counts) == 0 {
			log.Debug("Nothing to flush")
			flushed++
			continue
		}

		spans := w.toWrite[i].Sampler.GetSamples(w.toWrite[i].Stats, w.quantiles)

		log.Infof("Writer flush to the API, %d spans", len(spans))

		payload := model.Payload{
			APIKey: "424242", // FIXME, this should go in a config file
			Spans:  spans,
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
		log.Info("Flushed one payload")
		flushed++
	}

	if flushed != 0 {
		w.bufLock.Lock()
		w.toWrite = w.toWrite[flushed:]
		w.lastFlush = model.Now()
		w.bufLock.Unlock()
	} else {
		log.Warnf("Could not flush, still %d payloads to be flushed", maxBuf)
	}
}

package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func NewTestWriter() *Writer {
	exit := make(chan struct{})
	var exitGroup sync.WaitGroup

	quantiles := []float64{0.42, 0.99}
	inSpans := make(chan model.Span)
	inStats := make(chan model.StatsBucket)

	return NewWriter(
		"http://localhost:8080",
		quantiles,
		inSpans,
		inStats,
		exit,
		&exitGroup,
	)
}

// Very high-level stupid testing

func TestWriterCanHandleSpans(t *testing.T) {
	w := NewTestWriter()
	w.Start()

	w.inSpans <- model.Span{}
}

func TestWriterCanHandleStats(t *testing.T) {
	w := NewTestWriter()
	w.Start()

	w.inStats <- model.StatsBucket{}
}

func TestWriterExitsGracefully(t *testing.T) {
	w := NewTestWriter()
	w.Start()

	// And now try to stop it in a given time, by closing the exit channel
	timer := time.NewTimer(100 * time.Millisecond).C
	receivedExit := make(chan struct{}, 1)
	go func() {
		close(w.exit)
		w.exitGroup.Wait()
		close(receivedExit)
	}()
	for {
		select {
		case <-receivedExit:
			return
		case <-timer:
			t.Fatal("Writer did not exit in time")
		}
	}
}

// Testing the real logic of the writer
func TestWriterBufferFlush(t *testing.T) {
	assert := assert.New(t)

	// Create a fake API for the writer
	fakeApi := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer fakeApi.Close()

	w := NewTestWriter()
	w.Start()

	// should be ready to handle spans with one buffer
	assert.Equal(1, len(w.toWrite))
	assert.True(w.toWrite[0].Sampler.IsEmpty())

	// add some spans, don't test the sampler logic, already done in its file
	testSpans := []model.Span{
		model.Span{TraceID: 0, SpanID: 1},
		model.Span{TraceID: 1, SpanID: 2},
	}

	for _, span := range testSpans {
		w.inSpans <- span
	}

	// these spans should have been buffered in the sampler, toWrite is dangerously accessed by other routines
	w.bufLock.Lock()
	samplerEmpty := w.toWrite[0].Sampler.IsEmpty()
	w.bufLock.Unlock()
	assert.False(samplerEmpty, "Sampler is empty, spans got lost?")
	// sending a stats bucket should trigger a flush for that buffer
	lastFlush := w.lastFlush

	stats := model.NewStatsBucket(model.Now())
	stats.Duration = 100000
	w.inStats <- stats

	// wait for a flush
	// although, HTTP endpoint is down so we have 2 buffers now waiting
	ticker := time.NewTicker(10 * time.Millisecond).C
	loop := 0
	maxFlushWait := 10
	bufLen := 0
	for range ticker {
		// toWrite is dangerously written to by other routines
		w.bufLock.Lock()
		bufLen = len(w.toWrite)
		w.bufLock.Unlock()
		if bufLen > 1 || loop >= maxFlushWait {
			break
		}
		loop++
	}
	assert.Equal(2, len(w.toWrite), "Did not see failed flush in time")

	// now start our HTTPServer and send stuff to it
	fakeApi.Start()
	// point our writer to it
	// We have to stop the writer so that we don't get a race when we change w.endpoint
	close(w.exit)
	w.exitGroup.Wait()
	w.endpoint = fakeApi.URL + "/api/v0.1"
	w.Start()

	// Reflush, manually!
	w.Flush()
	// verify that we flushed!!
	assert.True(lastFlush < w.lastFlush)
	// FIXME test the data that has been flushed to the API
}

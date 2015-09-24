package main

import (
	"fmt"
	"io/ioutil"
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

	inBuckets := make(chan ConcentratorBucket)

	return NewWriter(
		NewAPIEndpoint("http://localhost:8080"),
		inBuckets,
		exit,
		&exitGroup,
	)
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

func getTestConcentratorBucket() ConcentratorBucket {
	now := model.Now()
	bucketSize := time.Duration(5 * time.Second).Nanoseconds()
	cb := newBucket(now, bucketSize)

	testSpans := []model.Span{
		model.Span{TraceID: 0, SpanID: 1},
		model.Span{TraceID: 1, SpanID: 2},
	}
	for _, s := range testSpans {
		cb.handleSpan(s)
	}

	return cb
}

// Testing the real logic of the writer
func TestWriterBufferFlush(t *testing.T) {
	assert := assert.New(t)

	// Create a fake API for the writer
	receivedData := false
	fakeAPI := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		fmt.Println(string(b))
		receivedData = true
		w.WriteHeader(200)
	}))
	defer fakeAPI.Close()

	w := NewTestWriter()
	w.Start()

	// light the fire by sending a bucket
	w.inBuckets <- getTestConcentratorBucket()

	// the bucket should be added to our queue pretty fast
	// HTTP endpoint is down so the bucket should stay in the queue
	ticker := time.NewTicker(10 * time.Millisecond).C
	loop := 0
	maxFlushWait := 10
	buckets := 0
	for range ticker {
		// toWrite is dangerously written to by other routines
		w.mu.Lock()
		buckets = len(w.bucketsToWrite)
		w.mu.Unlock()
		if buckets > 1 || loop >= maxFlushWait {
			break
		}
		loop++
	}
	assert.Equal(1, buckets, "New bucket was not added to the flush queue, broken pipes?")

	// now start our HTTPServer and send stuff to it
	fakeAPI.Start()
	// point our writer to it
	// We have to stop the writer so that we don't get a race when we change w.endpoint
	close(w.exit)
	w.exitGroup.Wait()
	w.endpoint = NewAPIEndpoint(fakeAPI.URL + "/api/v0.1")
	w.Start()

	// Reflush, manually! synchronous
	w.Flush()
	// verify that we flushed!!
	assert.True(receivedData)
}

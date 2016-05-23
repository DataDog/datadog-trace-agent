package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

func NewTestWriter() *Writer {
	conf := config.NewDefaultAgentConfig()
	conf.APIKey = "9d6e1075bb75e28ea6e720a4561f6b6d"
	conf.APIEndpoint = "http://localhost:8080"

	return NewWriter(conf, make(chan model.ServicesMetadata))
}

func TestWriterExitsGracefully(t *testing.T) {
	w := NewTestWriter()
	w.Start()

	// And now try to stop it in a given time, by closing the exit channel
	timer := time.NewTimer(100 * time.Millisecond).C
	receivedExit := make(chan struct{}, 1)
	go func() {
		close(w.exit)
		w.wg.Wait()
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

func getTestStatsBuckets() []model.StatsBucket {
	now := model.Now()
	bucketSize := time.Duration(5 * time.Second).Nanoseconds()
	sb := model.NewStatsBucket(now, bucketSize)

	testSpans := []model.Span{
		model.Span{TraceID: 0, SpanID: 1},
		model.Span{TraceID: 1, SpanID: 2},
	}
	for _, s := range testSpans {
		sb.HandleSpan(s, DefaultAggregators)
	}

	return []model.StatsBucket{sb}
}

// Testing the real logic of the writer
func TestWriterFlush(t *testing.T) {
	// Create a fake API for the writer
	receivedData := make(chan struct{}, 1)
	testAPI := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		fmt.Println(string(b))
		receivedData <- struct{}{}
		w.WriteHeader(200)
	}))
	defer testAPI.Close()
	testAPI.Start()

	// Start our writer with the test API
	conf := config.NewDefaultAgentConfig()
	conf.APIKey = "9d6e1075bb75e28ea6e720a4561f6b6d"
	conf.APIEndpoint = testAPI.URL + "/api/v0.1"
	w := NewWriter(conf, make(chan model.ServicesMetadata))
	w.Start()

	// light the fire by sending a bucket
	w.in <- model.AgentPayload{Stats: getTestStatsBuckets()}

	// Reflush, manually! synchronous
	w.Flush()
	timeout := make(chan struct{}, 1)
	go func() {
		time.Sleep(time.Second)
		timeout <- struct{}{}
	}()

	select {
	case <-timeout:
		t.Fatal("did not receive http payload in time")
	case <-receivedData:
	}
}

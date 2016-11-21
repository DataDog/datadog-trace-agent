package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
	"github.com/ugorji/go/codec"
)

// getTestSpan returns a Span with different fields set
func getTestSpan() model.Span {
	return model.Span{
		TraceID:  42,
		SpanID:   52,
		ParentID: 42,
		Type:     "web",
		Service:  "fennel_IS amazing!",
		Name:     "something &&<@# that should be a metric!",
		Resource: "NOT touched because it is going to be hashed",
		Start:    time.Now().UnixNano(),
		Duration: time.Second.Nanoseconds(),
		Meta:     map[string]string{"http.host": "192.168.0.1"},
		Metrics:  map[string]float64{"http.monitor": 41.99},
	}
}

// getTestTrace returns a []Trace that is composed by ``traceN`` number
// of traces, each one composed by ``size`` number of spans.
func getTestTrace(traceN, size int) []model.Trace {
	traces := []model.Trace{}

	for i := 0; i < traceN; i++ {
		trace := model.Trace{}
		for j := 0; j < size; j++ {
			trace = append(trace, getTestSpan())
		}
		traces = append(traces, trace)
	}
	return traces
}

func TestReceiverTraces(t *testing.T) {
	assert := assert.New(t)

	// get the default configuration
	defaultConfig := config.NewDefaultAgentConfig()

	// receiver just here so that we can attach the handler
	r := NewHTTPReceiver(defaultConfig)
	server := httptest.NewServer(
		http.HandlerFunc(httpHandleWithVersion(v02, r.handleTraces)),
	)
	defer server.Close()

	// send traces to that endpoint without a content-type
	traces := getTestTrace(1, 1)
	data, err := json.Marshal(traces)
	assert.Nil(err)
	req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer(data))
	assert.Nil(err)

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.Nil(err)
	assert.Equal(200, resp.StatusCode)

	defer resp.Body.Close()

	// now we should be able to read the trace data
	select {
	case rt := <-r.traces:
		assert.Len(rt, 1)
		span := rt[0]
		assert.Equal(uint64(42), span.TraceID)
		assert.Equal(uint64(52), span.SpanID)
		assert.Equal("fennel_is_amazing", span.Service)
		assert.Equal("something_that_should_be_a_metric", span.Name)
		assert.Equal("NOT touched because it is going to be hashed", span.Resource)
		assert.Equal("192.168.0.1", span.Meta["http.host"])
		assert.Equal(41.99, span.Metrics["http.monitor"])
	default:
		t.Fatalf("no data received")
	}
}

func TestReceiverTracesJSON(t *testing.T) {
	assert := assert.New(t)

	// get the default configuration
	defaultConfig := config.NewDefaultAgentConfig()

	// receiver just here so that we can attach the handler
	r := NewHTTPReceiver(defaultConfig)
	server := httptest.NewServer(
		http.HandlerFunc(httpHandleWithVersion(v02, r.handleTraces)),
	)
	defer server.Close()

	// send traces to that endpoint using the JSON content-type
	traces := getTestTrace(1, 1)
	data, err := json.Marshal(traces)
	assert.Nil(err)
	req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer(data))
	assert.Nil(err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.Nil(err)
	assert.Equal(200, resp.StatusCode)

	defer resp.Body.Close()

	// now we should be able to read the trace data
	select {
	case rt := <-r.traces:
		assert.Len(rt, 1)
		span := rt[0]
		assert.Equal(uint64(42), span.TraceID)
		assert.Equal(uint64(52), span.SpanID)
		assert.Equal("fennel_is_amazing", span.Service)
		assert.Equal("something_that_should_be_a_metric", span.Name)
		assert.Equal("NOT touched because it is going to be hashed", span.Resource)
		assert.Equal("192.168.0.1", span.Meta["http.host"])
		assert.Equal(41.99, span.Metrics["http.monitor"])
	default:
		t.Fatalf("no data received")
	}
}

func TestReceiverTracesMsgpack(t *testing.T) {
	assert := assert.New(t)
	var mh codec.MsgpackHandle

	// get the default configuration
	defaultConfig := config.NewDefaultAgentConfig()

	// receiver just here so that we can attach the handler
	r := NewHTTPReceiver(defaultConfig)
	server := httptest.NewServer(
		http.HandlerFunc(httpHandleWithVersion(v02, r.handleTraces)),
	)
	defer server.Close()

	// send traces to that endpoint using the JSON content-type
	traces := getTestTrace(1, 1)
	var data []byte
	enc := codec.NewEncoderBytes(&data, &mh)
	err := enc.Encode(traces)
	assert.Nil(err)
	req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer(data))
	assert.Nil(err)
	req.Header.Set("Content-Type", "application/msgpack")

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.Nil(err)
	assert.Equal(200, resp.StatusCode)

	defer resp.Body.Close()

	// now we should be able to read the trace data
	select {
	case rt := <-r.traces:
		assert.Len(rt, 1)
		span := rt[0]
		assert.Equal(uint64(42), span.TraceID)
		assert.Equal(uint64(52), span.SpanID)
		assert.Equal("fennel_is_amazing", span.Service)
		assert.Equal("something_that_should_be_a_metric", span.Name)
		assert.Equal("NOT touched because it is going to be hashed", span.Resource)
		assert.Equal("192.168.0.1", span.Meta["http.host"])
		assert.Equal(41.99, span.Metrics["http.monitor"])
	default:
		t.Fatalf("no data received")
	}
}

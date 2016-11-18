package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
	"github.com/ugorji/go/codec"
)

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

	// send traces to that endpoint
	traces := []model.Trace{
		model.Trace{
			model.Span{
				TraceID:  42,
				SpanID:   52,
				Service:  "fennel_IS amazing!",
				Name:     "something &&<@# that should be a metric!",
				Resource: "NOT touched because it is going to be hashed",
				Start:    time.Now().UnixNano(),
				Duration: time.Second.Nanoseconds(),
			},
		},
	}
	data, err := json.Marshal(traces)
	assert.Nil(err)
	req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer(data))
	assert.Nil(err)

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.Nil(err)

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
	default:
		t.Fatalf("no data received")
	}
}

func BenchmarkDecoderJSON(b *testing.B) {
	// preparing the environment
	simpleTrace, _ := ioutil.ReadFile("../simple_trace.json")
	complexTrace, _ := ioutil.ReadFile("../complex_trace.json")
	benchmarks := []struct {
		name string
		file []byte
	}{
		{"JSON simple trace", simpleTrace},
		{"JSON complex trace", complexTrace},
	}

	// benchmark
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				var spans []model.Span
				bodyBuffer := bytes.NewReader(bm.file)
				decoder := json.NewDecoder(bodyBuffer)

				err := decoder.Decode(&spans)
				if err != nil {
					b.Fatalf("Cannot decode the given stream: %s", err)
				}
			}
		})
	}
}

func BenchmarkDecoderMessagePack(b *testing.B) {
	// preparing the environment
	var mh codec.MsgpackHandle
	simpleTrace, _ := ioutil.ReadFile("../simple_trace.msgpack")
	complexTrace, _ := ioutil.ReadFile("../complex_trace.msgpack")
	benchmarks := []struct {
		name string
		file []byte
	}{
		{"MessagePack simple trace", simpleTrace},
		{"MessagePack complex trace", complexTrace},
	}

	// benchmark
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				var spans []model.Span
				bodyBuffer := bytes.NewReader(bm.file)
				decoder := codec.NewDecoder(bodyBuffer, &mh)

				err := decoder.Decode(&spans)
				if err != nil {
					b.Fatalf("Cannot decode the given stream: %s", err)
				}
			}
		})
	}
}

package model

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/ugorji/go/codec"
)

// getTestTrace returns a Trace with a single Span
func getTestTrace() []Trace {
	return []Trace{
		Trace{
			Span{
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
			},
		},
	}
}

func getJSONPayload() io.Reader {
	data, _ := json.Marshal(getTestTrace())
	return bytes.NewReader(data)
}

func getMsgpackPayload() io.Reader {
	var data []byte
	enc := codec.NewEncoderBytes(&data, &codec.MsgpackHandle{})
	enc.Encode(getTestTrace())
	return bytes.NewReader(data)
}

func TestMain(m *testing.M) {
	flag.Parse()

	// neutralize logs for tests
	config.NewLoggerLevelCustom("critical", "")

	os.Exit(m.Run())
}

func TestDecoders(t *testing.T) {
	assert := assert.New(t)
	testCases := []struct {
		payload io.Reader
		decoder ClientDecoder
	}{
		{payload: getJSONPayload(), decoder: newJSONDecoder()},
		{payload: getMsgpackPayload(), decoder: newMsgpackDecoder()},
	}

	for _, tc := range testCases {
		var traces []Trace
		err := tc.decoder.Decode(tc.payload, &traces)

		assert.Nil(err)
		assert.Len(traces, 1)
		trace := traces[0]
		assert.Len(trace, 1)
		span := trace[0]
		assert.Equal(uint64(42), span.TraceID)
		assert.Equal(uint64(52), span.SpanID)
		assert.Equal("fennel_IS amazing!", span.Service)
		assert.Equal("something &&<@# that should be a metric!", span.Name)
		assert.Equal("NOT touched because it is going to be hashed", span.Resource)
		assert.Equal("192.168.0.1", span.Meta["http.host"])
		assert.Equal(41.99, span.Metrics["http.monitor"])
	}
}

func TestDecodersReusable(t *testing.T) {
	assert := assert.New(t)
	testCases := []struct {
		firstPayload  io.Reader
		secondPayload io.Reader
		decoder       ClientDecoder
	}{
		{firstPayload: getJSONPayload(), secondPayload: getJSONPayload(), decoder: newJSONDecoder()},
		{firstPayload: getMsgpackPayload(), secondPayload: getMsgpackPayload(), decoder: newMsgpackDecoder()},
	}

	for _, tc := range testCases {
		// first decoding
		var firstTraces []Trace
		err := tc.decoder.Decode(tc.firstPayload, &firstTraces)
		assert.Nil(err)

		// second decoding
		var secondTraces []Trace
		err = tc.decoder.Decode(tc.secondPayload, &secondTraces)
		assert.Nil(err)

		assert.Len(secondTraces, 1)
		trace := secondTraces[0]
		assert.Len(trace, 1)
		span := trace[0]
		assert.Equal(uint64(42), span.TraceID)
		assert.Equal(uint64(52), span.SpanID)
		assert.Equal("fennel_IS amazing!", span.Service)
		assert.Equal("something &&<@# that should be a metric!", span.Name)
		assert.Equal("NOT touched because it is going to be hashed", span.Resource)
		assert.Equal("192.168.0.1", span.Meta["http.host"])
		assert.Equal(41.99, span.Metrics["http.monitor"])

		// the two data structures should be different because of the timestamps
		assert.False(reflect.DeepEqual(firstTraces, secondTraces))
	}
}

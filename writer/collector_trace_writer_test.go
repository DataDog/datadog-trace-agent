package writer

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/model/collector"
	"github.com/DataDog/datadog-trace-agent/testutil"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestCollectorTraceWriter(t *testing.T) {
	t.Run("payload flushing", func(t *testing.T) {
		assert := assert.New(t)

		// Create a trace writer, its incoming channel and the endpoint that receives the payloads
		traceWriter, traceChannel, testEndpoint := testCollectorTraceWriter()
		// Set a maximum of 4 spans per payload
		traceWriter.conf.MaxSpansPerPayload = 4
		traceWriter.Start()

		traces := append(testutil.GetTestTrace(2, 2, true), testutil.GetTestTrace(3, 5, true)...)
		for i := range traces {
			traceChannel <- &traces[i]
		}

		// Stop the trace writer to force everything to flush
		close(traceChannel)
		traceWriter.Stop()

		assert.Len(testEndpoint.SuccessPayloads(), 4, "We expected 4 different payloads")
		assertCollectorPayloads(assert, traceWriter, traces, testEndpoint.SuccessPayloads())
	})
}

func assertCollectorPayloads(assert *assert.Assertions, traceWriter *CollectorTraceWriter, expectedTraces []model.Trace, payloads []*payload) {
	expectedTraceIdx := 0

	for _, payload := range payloads {
		var sendTracesRequest collector.SendTracesRequest
		assert.NoError(proto.Unmarshal(payload.bytes, &sendTracesRequest), "Unmarshalling should work correctly")

		numSpans := 0

		for _, chunk := range sendTracesRequest.Chunks {
			numSpans += len(chunk.Spans)

			assert.Equal(testHostName, chunk.Hostname, "Hostnames should match")

			expectedTrace := expectedTraces[expectedTraceIdx]
			assert.Equal(len(expectedTrace), len(chunk.Spans), "Trace length should match")

			for expectedSpanIdx := range expectedTrace {
				if !assert.True(proto.Equal(expectedTrace[expectedSpanIdx], chunk.Spans[expectedSpanIdx]),
					"Unmarshalled span should match expectation at trace index %d and span index %d", expectedTraceIdx, expectedSpanIdx) {
					return
				}
			}

			expectedTraceIdx++
		}

		// If there's more than 1 chunk in this payload, don't let it go over the limit. Otherwise,
		// a single trace+transaction combination is allows to go over the limit.
		if len(sendTracesRequest.Chunks) > 1 {
			assert.True(numSpans <= traceWriter.conf.MaxSpansPerPayload)
		}
	}
}

func testCollectorTraceWriter() (*CollectorTraceWriter, chan *model.Trace, *testEndpoint) {
	traceChannel := make(chan *model.Trace)
	conf := &config.AgentConfig{
		Hostname:          testHostName,
		DefaultEnv:        testEnv,
		TraceWriterConfig: writerconfig.DefaultTraceWriterConfig(),
	}
	traceWriter := NewCollectorTraceWriter(conf, traceChannel)
	testEndpoint := &testEndpoint{}
	traceWriter.sender.setEndpoint(testEndpoint)

	return traceWriter, traceChannel, testEndpoint
}

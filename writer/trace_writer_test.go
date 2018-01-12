package writer

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

var testHostName = "testhost"
var testEnv = "testenv"

func TestTraceWriter_TraceHandling(t *testing.T) {
	assert := assert.New(t)

	// Given a trace writer, its incoming channel and the endpoint that receives the payloads
	traceWriter, traceChannel, testEndpoint, _ := testTraceWriter()
	traceWriter.conf.FlushPeriod = 100 * time.Millisecond

	traceWriter.Start()

	// Given a set of 6 test traces
	testTraces := []model.Trace{
		fixtures.RandomTrace(3, 1),
		fixtures.RandomTrace(1, 3),
		fixtures.RandomTrace(9, 3),
		fixtures.RandomTrace(1, 3),
		fixtures.RandomTrace(3, 5),
		fixtures.RandomTrace(3, 1),
	}

	// When sending those 6 traces
	for _, trace := range testTraces {
		traceCopy := trace
		traceChannel <- &traceCopy
	}

	// And then waiting for more than flush period
	time.Sleep(2 * traceWriter.conf.FlushPeriod)

	// And then sending another trace
	afterTrace := fixtures.RandomTrace(2, 2)
	testTraces = append(testTraces, afterTrace)
	traceChannel <- &afterTrace

	// And stopping trace writer before flush ticker ticks (should still flush on exit though)
	close(traceChannel)
	traceWriter.Stop()

	// Then the endpoint should have received 2 payloads, containing all sent traces
	expectedHeaders := map[string]string{
		"X-Datadog-Reported-Languages": strings.Join(info.Languages(), "|"),
		"Content-Type":                 "application/x-protobuf",
		"Content-Encoding":             "identity",
	}

	assert.Len(testEndpoint.SuccessPayloads, 2, "There should be 2 payloads")
	assertPayloads(assert, traceWriter, expectedHeaders, testTraces, testEndpoint.SuccessPayloads)
}

func TestTraceWriter_BigTraceHandling(t *testing.T) {
	assert := assert.New(t)

	// Given a trace writer, its incoming channel and the endpoint that receives the payloads
	traceWriter, traceChannel, testEndpoint, _ := testTraceWriter()
	traceWriter.conf.MaxSpansPerPayload = 3

	traceWriter.Start()

	// Given a set of 6 test traces
	testTraces := []model.Trace{
		fixtures.RandomTrace(3, 1),
		fixtures.RandomTrace(1, 3),
		fixtures.RandomTrace(9, 3),
		fixtures.RandomTrace(1, 3),
		fixtures.RandomTrace(3, 5),
		fixtures.RandomTrace(3, 1),
	}

	// When sending those 6 traces
	for _, trace := range testTraces {
		traceCopy := trace
		traceChannel <- &traceCopy
	}

	// And stopping trace writer
	close(traceChannel)
	traceWriter.Stop()

	// Then the endpoint should have received several payloads, containing all sent traces but not going over
	// the span limit.
	expectedHeaders := map[string]string{
		"X-Datadog-Reported-Languages": strings.Join(info.Languages(), "|"),
		"Content-Type":                 "application/x-protobuf",
		"Content-Encoding":             "identity",
	}

	numSpans := 0

	for _, trace := range testTraces {
		numSpans += len(trace)
	}

	expectedNumPayloads := int(math.Ceil(float64(numSpans) / float64(traceWriter.conf.MaxSpansPerPayload)))
	assert.Len(testEndpoint.SuccessPayloads, expectedNumPayloads, "There should be more than 1 payload")
	assertPayloads(assert, traceWriter, expectedHeaders, testTraces, testEndpoint.SuccessPayloads)
}

func TestTraceWriter_UpdateInfoHandling(t *testing.T) {
	assert := assert.New(t)

	// Given a trace writer, its incoming channel and the endpoint that receives the payloads
	traceWriter, traceChannel, testEndpoint, statsClient := testTraceWriter()
	traceWriter.conf.FlushPeriod = 100 * time.Millisecond
	traceWriter.conf.UpdateInfoPeriod = 100 * time.Millisecond

	traceWriter.Start()

	expectedNumPayloads := int64(0)
	expectedNumSpans := int64(0)
	expectedNumTraces := int64(0)
	expectedNumBytes := int64(0)
	expectedNumErrors := int64(0)
	expectedMinNumRetries := int64(0)

	// When sending 1 payload with 3 traces
	expectedNumPayloads++
	payload1Traces := []model.Trace{
		fixtures.RandomTrace(3, 1),
		fixtures.RandomTrace(1, 3),
		fixtures.RandomTrace(9, 3),
	}
	for _, trace := range payload1Traces {
		expectedNumTraces++
		expectedNumSpans += int64(len(trace))
		traceCopy := trace
		traceChannel <- &traceCopy
	}
	expectedNumBytes += calculateTracePayloadSize(payload1Traces)

	// And waiting for twice the flush period to trigger payload sending and info updating
	time.Sleep(2 * traceWriter.conf.FlushPeriod)

	// And then sending a second payload with other 3 traces
	expectedNumPayloads++
	payload2Traces := []model.Trace{
		fixtures.RandomTrace(3, 1),
		fixtures.RandomTrace(1, 3),
		fixtures.RandomTrace(9, 3),
	}
	for _, trace := range payload2Traces {
		expectedNumTraces++
		expectedNumSpans += int64(len(trace))
		traceCopy := trace
		traceChannel <- &traceCopy
	}
	expectedNumBytes += calculateTracePayloadSize(payload2Traces)

	// And waiting for twice the flush period to trigger payload sending and info updating
	time.Sleep(2 * traceWriter.conf.FlushPeriod)

	// And then sending a third payload with other 3 traces with an errored out endpoint
	testEndpoint.Err = fmt.Errorf("non retriable error")
	expectedNumErrors++
	payload3Traces := []model.Trace{
		fixtures.RandomTrace(3, 1),
		fixtures.RandomTrace(1, 3),
		fixtures.RandomTrace(9, 3),
	}
	for _, trace := range payload3Traces {
		expectedNumTraces++
		expectedNumSpans += int64(len(trace))
		traceCopy := trace
		traceChannel <- &traceCopy
	}
	expectedNumBytes += calculateTracePayloadSize(payload3Traces)

	// And waiting for twice the flush period to trigger payload sending and info updating
	time.Sleep(2 * traceWriter.conf.FlushPeriod)

	// And then sending a fourth payload with other 3 traces with an errored out endpoint but retriable
	testEndpoint.Err = &RetriableError{
		err:      fmt.Errorf("non retriable error"),
		endpoint: testEndpoint,
	}
	expectedMinNumRetries++
	payload4Traces := []model.Trace{
		fixtures.RandomTrace(3, 1),
		fixtures.RandomTrace(1, 3),
		fixtures.RandomTrace(9, 3),
	}
	for _, trace := range payload4Traces {
		expectedNumTraces++
		expectedNumSpans += int64(len(trace))
		traceCopy := trace
		traceChannel <- &traceCopy
	}
	expectedNumBytes += calculateTracePayloadSize(payload4Traces)

	// And waiting for twice the flush period to trigger payload sending and info updating
	time.Sleep(2 * traceWriter.conf.FlushPeriod)

	close(traceChannel)
	traceWriter.Stop()

	// Then we expect some counts to have been sent to the stats client for each update tick (there should have been
	// at least 3 ticks)
	countSummaries := statsClient.GetCountSummaries()

	// Payload counts
	payloadSummary := countSummaries["datadog.trace_agent.trace_writer.payloads"]
	assert.True(len(payloadSummary.Calls) >= 3, "There should have been multiple payload count calls")
	assert.Equal(expectedNumPayloads, payloadSummary.Sum)

	// Traces counts
	tracesSummary := countSummaries["datadog.trace_agent.trace_writer.traces"]
	assert.True(len(tracesSummary.Calls) >= 3, "There should have been multiple traces count calls")
	assert.Equal(expectedNumTraces, tracesSummary.Sum)

	// Spans counts
	spansSummary := countSummaries["datadog.trace_agent.trace_writer.spans"]
	assert.True(len(spansSummary.Calls) >= 3, "There should have been multiple spans count calls")
	assert.Equal(expectedNumSpans, spansSummary.Sum)

	// Bytes counts
	bytesSummary := countSummaries["datadog.trace_agent.trace_writer.bytes"]
	assert.True(len(bytesSummary.Calls) >= 3, "There should have been multiple bytes count calls")
	assert.Equal(expectedNumBytes, bytesSummary.Sum)

	// Retry counts
	retriesSummary := countSummaries["datadog.trace_agent.trace_writer.retries"]
	assert.True(len(retriesSummary.Calls) >= 3, "There should have been multiple retries count calls")
	assert.True(retriesSummary.Sum >= expectedMinNumRetries)

	// Error counts
	errorsSummary := countSummaries["datadog.trace_agent.trace_writer.errors"]
	assert.True(len(errorsSummary.Calls) >= 3, "There should have been multiple errors count calls")
	assert.Equal(expectedNumErrors, errorsSummary.Sum)
}

func calculateTracePayloadSize(traces []model.Trace) int64 {
	apiTraces := make([]*model.APITrace, len(traces))

	for i, trace := range traces {
		apiTraces[i] = trace.APITrace()
	}

	tracePayload := model.TracePayload{
		HostName: testHostName,
		Env:      testEnv,
		Traces:   apiTraces,
	}

	serialized, _ := proto.Marshal(&tracePayload)

	return int64(len(serialized))
}

func assertPayloads(assert *assert.Assertions, traceWriter *TraceWriter, expectedHeaders map[string]string,
	expectedTraces []model.Trace, payloads []Payload) {

	seenAPITraces := []*model.APITrace(nil)
	for _, payload := range payloads {
		assert.Equal(expectedHeaders, payload.Headers, "Payload headers should match expectation")

		var tracePayload model.TracePayload
		assert.NoError(proto.Unmarshal(payload.Bytes, &tracePayload), "Unmarshalling should work correctly")

		assert.Equal(testEnv, tracePayload.Env, "Envs should match")
		assert.Equal(testHostName, tracePayload.HostName, "Hostnames should match")

		numSpans := 0

		for _, seenAPITrace := range tracePayload.Traces {
			numSpans += len(seenAPITrace.Spans)
			seenAPITraces = append(seenAPITraces, seenAPITrace)
		}

		assert.True(numSpans <= traceWriter.conf.MaxSpansPerPayload)
	}

	traceCount := 0

	for _, trace := range expectedTraces {
		for len(trace) > 0 {
			seenAPITrace := seenAPITraces[traceCount]
			seenAPITraceNumSpans := len(seenAPITrace.Spans)

			expectedTrace := trace[:seenAPITraceNumSpans]

			if seenAPITraceNumSpans < len(trace) {
				trace = trace[seenAPITraceNumSpans:]
			} else {
				trace = nil
			}

			expectedTraceAPI := expectedTrace.APITrace()

			assert.True(proto.Equal(expectedTraceAPI, seenAPITraces[traceCount]),
				"Unmarshalled trace payload should match expectation at index %d", traceCount)
			traceCount++
		}
	}

}

func testTraceWriter() (*TraceWriter, chan *model.Trace, *TestEndpoint, *statsd.TestStatsClient) {
	traceChannel := make(chan *model.Trace)
	transactionChannel := make(chan *model.Span)
	traceWriter := NewTraceWriter(&config.AgentConfig{HostName: testHostName, DefaultEnv: testEnv}, traceChannel, transactionChannel)
	testEndpoint := &TestEndpoint{}
	traceWriter.BaseWriter.payloadSender.setEndpoint(testEndpoint)
	testStatsClient := &statsd.TestStatsClient{}
	traceWriter.conf.StatsClient = testStatsClient

	return traceWriter, traceChannel, testEndpoint, testStatsClient
}

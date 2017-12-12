package writer

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
)

func newBenchPayload(traces, spans, stats int) model.AgentPayload {
	payload := model.AgentPayload{
		HostName: "test.host",
		Env:      "test",
	}
	for i := 0; i < traces; i++ {
		var trace model.Trace
		for j := 0; j < spans/traces; j++ {
			span := fixtures.TestSpan()
			span.TraceID += uint64(i * spans)
			span.ParentID += uint64(i * spans)
			span.SpanID += uint64(i*spans + j)
			span.Start += int64(i*spans + j)
			trace = append(trace, span)
		}
		payload.Traces = append(payload.Traces, trace)
	}
	for i := 0; i < stats; i++ {
		payload.Stats = append(payload.Stats, fixtures.TestStatsBucket())
	}
	return payload
}

func BenchmarkEncodeAgentPayload(b *testing.B) {
	payload := newBenchPayload(10, 1000, 100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := model.EncodeAgentPayload(&payload); err != nil {
			b.Fatalf("error encoding payload: %v", b)
		}
	}
}

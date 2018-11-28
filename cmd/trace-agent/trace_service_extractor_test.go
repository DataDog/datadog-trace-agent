package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/stretchr/testify/assert"
)

func TestTracerServiceExtractor(t *testing.T) {
	assert := assert.New(t)

	testChan := make(chan agent.ServicesMetadata)
	testExtractor := NewTraceServiceExtractor(testChan)

	trace := agent.Trace{
		&agent.Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "service-a", Type: "type-a"},
		&agent.Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "service-b", Type: "type-b"},
		&agent.Span{TraceID: 1, SpanID: 3, ParentID: 1, Service: "service-c", Type: "type-c"},
		&agent.Span{TraceID: 1, SpanID: 4, ParentID: 3, Service: "service-c", Type: "ignore"},
	}

	trace.ComputeTopLevel()
	wt := agent.NewWeightedTrace(trace, trace[0])

	go func() {
		testExtractor.Process(wt)
	}()

	metadata := <-testChan

	// Result should only contain information derived from top-level spans
	assert.Equal(metadata, agent.ServicesMetadata{
		"service-a": {"app_type": "type-a"},
		"service-b": {"app_type": "type-b"},
		"service-c": {"app_type": "type-c"},
	})
}

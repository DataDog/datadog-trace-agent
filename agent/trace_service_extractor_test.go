package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestTracerServiceExtractor(t *testing.T) {
	assert := assert.New(t)

	testChan := make(chan model.ServicesMetadata)
	testExtractor := NewTraceServiceExtractor(testChan)

	trace := model.Trace{
		&model.Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "service-a", Type: "type-a"},
		&model.Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "service-b", Type: "type-b"},
		&model.Span{TraceID: 1, SpanID: 3, ParentID: 1, Service: "service-c", Type: "type-c"},
		&model.Span{TraceID: 1, SpanID: 4, ParentID: 3, Service: "service-c", Type: "ignore"},
	}

	trace.ComputeTopLevel()
	wt := model.NewWeightedTrace(trace, trace[0])

	go func() {
		testExtractor.Process(wt)
	}()

	metadata := <-testChan

	// Result should only contain information derived from top-level spans
	assert.Equal(metadata, model.ServicesMetadata{
		"service-a": {"app_type": "type-a"},
		"service-b": {"app_type": "type-b"},
		"service-c": {"app_type": "type-c"},
	})
}

package main

import (
	"github.com/DataDog/datadog-trace-agent/internal/agent"
)

// TraceServiceExtractor extracts service metadata from top-level spans
type TraceServiceExtractor struct {
	outServices chan<- agent.ServicesMetadata
}

// NewTraceServiceExtractor returns a new TraceServiceExtractor
func NewTraceServiceExtractor(out chan<- agent.ServicesMetadata) *TraceServiceExtractor {
	return &TraceServiceExtractor{out}
}

// Process extracts service metadata from top-level spans and sends it downstream
func (ts *TraceServiceExtractor) Process(t agent.WeightedTrace) {
	meta := make(agent.ServicesMetadata)

	for _, s := range t {
		if !s.TopLevel {
			continue
		}

		if _, ok := meta[s.Service]; ok {
			continue
		}

		if v := s.Type; len(v) > 0 {
			meta[s.Service] = map[string]string{agent.AppType: v}
		}
	}

	if len(meta) > 0 {
		ts.outServices <- meta
	}
}

package main

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	TraceIDBySpanID map[uint64]uint64
	SpansByTraceID  map[uint64][]model.Span
}

// NewSampler creates a new empty sampler
func NewSampler() Sampler {
	return Sampler{
		TraceIDBySpanID: map[uint64]uint64{},
		SpansByTraceID:  map[uint64][]model.Span{},
	}
}

// IsEmpty tells if the sampler contains no span
func (s Sampler) IsEmpty() bool {
	return len(s.TraceIDBySpanID) == 0
}

// AddSpan adds a span to the sampler internal momory
func (s Sampler) AddSpan(span model.Span) {
	s.TraceIDBySpanID[span.SpanID] = span.TraceID

	spans, ok := s.SpansByTraceID[span.TraceID]
	if !ok {
		spans = []model.Span{span}
	} else {
		spans = append(spans, span)
	}
	s.SpansByTraceID[span.TraceID] = spans
}

// GetSamples returns a list of representative spans to write
func (s *Sampler) GetSamples(sb model.StatsBucket, quantiles []float64) []model.Span {
	spanIDs := make([]uint64, len(sb.Distributions)*len(quantiles))
	// Look at the stats to find representative spans
	for _, d := range sb.Distributions {
		for _, q := range quantiles {
			_, sIDs := d.Summary.Quantile(q)

			if len(sIDs) > 0 { // TODO: not sure this condition is required
				spanIDs = append(spanIDs, sIDs[0])
			}
		}
	}

	// Then find the trace IDs thanks to a spanID -> traceID map
	traceIDSet := make(map[uint64]struct{})
	var token struct{}
	for _, spanID := range spanIDs {
		// spanIDs is pre-allocated, so it may contain zeros
		if spanID != 0 {
			traceID, ok := s.TraceIDBySpanID[spanID]
			if !ok {
				log.Errorf("SpanID reported by Quantiles not available in Sampler, SpanID=%d", spanID)
			} else {
				traceIDSet[traceID] = token
			}
		}
	}

	// Then get the traces (ie. set of spans) thanks to a traceID -> []spanID map
	spans := []model.Span{}
	for traceID := range traceIDSet {
		spans = append(spans, s.SpansByTraceID[traceID]...)
	}

	Statsd.Count("trace_agent.sampler.trace.total", int64(len(s.SpansByTraceID)), nil, 1)
	Statsd.Count("trace_agent.sampler.trace.kept", int64(len(traceIDSet)), nil, 1)
	Statsd.Count("trace_agent.sampler.span.total", int64(len(s.TraceIDBySpanID)), nil, 1)
	Statsd.Count("trace_agent.sampler.span.kept", int64(len(spans)), nil, 1)

	log.Infof("Sampled %d traces out of %d, %d spans out of %d",
		len(traceIDSet), len(s.SpansByTraceID), len(spans), len(s.TraceIDBySpanID))

	return spans
}

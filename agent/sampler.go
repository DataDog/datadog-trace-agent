package main

import (
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// FIXME[leo]: do not hardcode it maybe?
var DefaultQuantiles = [...]float64{0, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 1}

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
func (s *Sampler) GetSamples(sb model.StatsBucket) []model.Span {
	startTime := time.Now()
	spanIDs := make([]uint64, len(sb.Distributions)*len(DefaultQuantiles))

	// Look at the stats to find representative spans
	for _, d := range sb.Distributions {
		for _, q := range DefaultQuantiles {
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

	execTime := time.Since(startTime)
	log.Infof("Sampled %d traces out of %d, %d spans out of %d, in %s",
		len(traceIDSet), len(s.SpansByTraceID), len(spans), len(s.TraceIDBySpanID), execTime)

	Statsd.Gauge("trace_agent.sampler.sample_duration", execTime.Seconds(), nil, 1)

	return spans
}

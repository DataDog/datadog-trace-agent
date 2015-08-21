package main

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	TraceIDBySpanID map[model.SID]model.TID
	SpansByTraceID  map[model.TID][]*model.Span
}

// NewSampler creates a new empty sampler
func NewSampler() *Sampler {
	return &Sampler{
		TraceIDBySpanID: map[model.SID]model.TID{},
		SpansByTraceID:  map[model.TID][]*model.Span{},
	}
}

// IsEmpty tells if the sampler contains no span
func (s *Sampler) IsEmpty() bool {
	return len(s.TraceIDBySpanID) == 0
}

// AddSpan adds a span to the sampler internal momory
func (s *Sampler) AddSpan(span *model.Span) {
	s.TraceIDBySpanID[span.SpanID] = span.TraceID
	spans, ok := s.SpansByTraceID[span.TraceID]
	if !ok {
		s.SpansByTraceID[span.TraceID] = []*model.Span{span}
	} else {
		s.SpansByTraceID[span.TraceID] = append(spans, span)
	}
}

// GetSamples returns a list of representative spans to write
func (s *Sampler) GetSamples(sb *model.StatsBucket, minSpanByDistribution int) []*model.Span {
	qn := float64(1) / float64(minSpanByDistribution-1)
	quantiles := make([]float64, minSpanByDistribution)
	for i := 0; i < minSpanByDistribution; i++ {
		quantiles[i] = float64(i) * qn
	}

	// Look at the stats to find representative spans
	spanIDs := []model.SID{}
	for _, c := range sb.Counts {
		d := c.Distribution
		if d != nil {
			for _, q := range quantiles {
				_, sIDs := d.Quantile(q)

				if len(sIDs) > 0 { // TODO: not sure this condition is required
					spanIDs = append(spanIDs, sIDs[0])
				}
			}
		}
	}

	// Then find the trace IDs thanks to a spanID -> traceID map
	traceIDSet := map[model.TID]interface{}{}
	for _, spanID := range spanIDs {
		traceIDSet[s.TraceIDBySpanID[spanID]] = nil
	}

	// Then get the traces (ie. set of spans) thanks to a traceID -> []spanID map
	spans := []*model.Span{}
	for traceID := range traceIDSet {
		spans = append(spans, s.SpansByTraceID[traceID]...)
	}

	log.Infof("Sampled %d traces out of %d, %d spans out of %d",
		len(traceIDSet), len(s.SpansByTraceID), len(spans), len(s.TraceIDBySpanID))

	return spans
}

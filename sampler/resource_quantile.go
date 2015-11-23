package sampler

import (
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

var DefaultAggregators = []string{"service", "resource"}

// Sampler chooses wich spans to write to the API
type ResourceQuantileSampler struct {
	stats           model.StatsBucket
	traceIDBySpanID map[uint64]uint64
	spansByTraceID  map[uint64][]model.Span

	conf *config.AgentConfig
	mu   sync.Mutex
}

// NewResourceQuantileSampler creates a ResourceQuantileSampler
func NewResourceQuantileSampler(conf *config.AgentConfig) *ResourceQuantileSampler {
	return &ResourceQuantileSampler{
		stats:           model.NewStatsBucket(0, 1),
		traceIDBySpanID: map[uint64]uint64{},
		spansByTraceID:  map[uint64][]model.Span{},
		conf:            conf,
	}
}

// AddSpan adds a span to the sampler internal momory
func (s *ResourceQuantileSampler) AddSpan(span model.Span) {
	s.mu.Lock()
	s.traceIDBySpanID[span.SpanID] = span.TraceID

	spans, ok := s.spansByTraceID[span.TraceID]
	if !ok {
		spans = []model.Span{span}
	} else {
		spans = append(spans, span)
	}
	s.spansByTraceID[span.TraceID] = spans

	s.stats.HandleSpan(span, DefaultAggregators)
	s.mu.Unlock()
}

func (s *ResourceQuantileSampler) Flush() []model.Span {
	s.mu.Lock()
	traceIDBySpanID := s.traceIDBySpanID
	spansByTraceID := s.spansByTraceID
	stats := s.stats
	s.traceIDBySpanID = map[uint64]uint64{}
	s.spansByTraceID = map[uint64][]model.Span{}
	s.stats = model.NewStatsBucket(0, 1)
	s.mu.Unlock()

	return s.GetSamples(traceIDBySpanID, spansByTraceID, stats)
}

func (s *ResourceQuantileSampler) GetSamples(
	traceIDBySpanID map[uint64]uint64, spansByTraceID map[uint64][]model.Span, stats model.StatsBucket,
) []model.Span {
	// We should merge them instead of picking a random one
	startTime := time.Now()
	spanIDs := make([]uint64, len(stats.Distributions)*len(s.conf.SamplerQuantiles))

	// Look at the stats to find representative spans
	for _, d := range stats.Distributions {
		for _, q := range s.conf.SamplerQuantiles {
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
			traceID, ok := traceIDBySpanID[spanID]
			if !ok {
				log.Errorf("Span not available in Sampler, SpanID=%d", spanID)
			} else {
				traceIDSet[traceID] = token
			}
		}
	}

	// Then get the traces (ie. set of spans) thanks to a traceID -> []spanID map
	spans := []model.Span{}
	for traceID := range traceIDSet {
		spans = append(spans, spansByTraceID[traceID]...)
	}

	// Statsd.Count("trace_agent.sampler.trace.total", int64(len(spansByTraceID)), nil, 1)
	// Statsd.Count("trace_agent.sampler.trace.kept", int64(len(traceIDSet)), nil, 1)
	// Statsd.Count("trace_agent.sampler.span.total", int64(len(traceIDBySpanID)), nil, 1)
	// Statsd.Count("trace_agent.sampler.span.kept", int64(len(spans)), nil, 1)

	execTime := time.Since(startTime)
	log.Infof("Sampled %d traces out of %d, %d spans out of %d, in %s",
		len(traceIDSet), len(spansByTraceID), len(spans), len(traceIDBySpanID), execTime)

	// Statsd.Gauge("trace_agent.sampler.sample_duration", execTime.Seconds(), nil, 1)

	return spans
}

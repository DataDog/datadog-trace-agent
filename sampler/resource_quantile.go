package sampler

import (
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/statsd"
)

var DefaultAggregators = []string{"service", "name", "resource"}

// ResourceQuantileSampler samples by selectic spans representative of each sampler quantiles for each resource from local statistics
type ResourceQuantileSampler struct {
	stats         model.StatsBucket
	traceBySpanID map[uint64]*model.Trace

	// counters
	spans  int
	traces int

	conf *config.AgentConfig
	mu   sync.Mutex
}

// NewResourceQuantileSampler creates a new ResourceQuantileSampler, ready to ingest spans
func NewResourceQuantileSampler(conf *config.AgentConfig) *ResourceQuantileSampler {
	sb := model.NewStatsBucket(0, 1, conf.LatencyResolution)
	// we need to keep samples for sampling
	sb.DistroSamples = true

	return &ResourceQuantileSampler{
		stats:         sb,
		traceBySpanID: map[uint64]*model.Trace{},
		conf:          conf,
	}
}

// AddSpan adds a span to the ResourceQuantileSampler internal memory
func (s *ResourceQuantileSampler) AddTrace(trace model.Trace) {
	s.mu.Lock()

	for _, span := range trace {
		s.traceBySpanID[span.SpanID] = &trace
		s.stats.HandleSpan(span, DefaultAggregators)
		s.spans++
	}
	s.traces++

	s.mu.Unlock()
}

// Flush returns representative spans based on GetSamples and reset its internal memory
func (s *ResourceQuantileSampler) Flush() []model.Trace {
	s.mu.Lock()
	traceBySpanID := s.traceBySpanID
	stats := s.stats
	spans := s.spans
	traces := s.traces

	s.traceBySpanID = map[uint64]*model.Trace{}
	s.stats = model.NewStatsBucket(0, 1, s.conf.LatencyResolution)
	s.stats.DistroSamples = true
	s.spans = 0
	s.traces = 0
	s.mu.Unlock()

	return s.GetSamples(traceBySpanID, stats, spans, traces)
}

// GetSamples returns interesting spans by picking a representative of each SamplerQuantiles of each resource
func (s *ResourceQuantileSampler) GetSamples(
	traceBySpanID map[uint64]*model.Trace, stats model.StatsBucket, spans, traces int,
) []model.Trace {

	startTime := time.Now()
	selected := make(map[*model.Trace]struct{})

	// Look at the stats to find representative spans
	for _, d := range stats.Distributions {
		for _, q := range s.conf.SamplerQuantiles {
			_, sIDs := d.Summary.Quantile(q)

			if len(sIDs) > 0 {
				t, ok := traceBySpanID[sIDs[0]]
				if ok {
					selected[t] = struct{}{}
				}
			}
		}
	}

	var kSpans int
	result := make([]model.Trace, 0, len(selected))
	for tptr := range selected {
		result = append(result, *tptr)
		kSpans += len(*tptr)
	}

	execTime := time.Since(startTime)
	log.Infof("sampler: selected %d traces (%.2f %%), %d spans (%.2f %%)",
		len(result), float64(len(result))*100/float64(traces), kSpans, float64(kSpans)*100/float64(spans))

	statsd.Client.Count("trace_agent.sampler.trace.total", int64(traces), nil, 1)
	statsd.Client.Count("trace_agent.sampler.trace.kept", int64(len(result)), nil, 1)
	statsd.Client.Count("trace_agent.sampler.span.total", int64(spans), nil, 1)
	statsd.Client.Count("trace_agent.sampler.span.kept", int64(kSpans), nil, 1)
	statsd.Client.Gauge("trace_agent.sampler.sample_duration", execTime.Seconds(), nil, 1)

	return result
}

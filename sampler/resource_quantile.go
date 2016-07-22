package sampler

import (
	"sync"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

// DefaultAggregators is the set if attributes to use for computing local statistics (that we use to get quantiles)
var DefaultAggregators = []string{"service", "name", "resource"}

// ResourceQuantileSampler samples by selectic spans representative of each sampler quantiles for each resource from local statistics
type ResourceQuantileSampler struct {
	stats         model.StatsBucket
	traceBySpanID map[uint64]*model.Trace

	conf *config.AgentConfig
	mu   sync.Mutex
}

// NewResourceQuantileSampler creates a new ResourceQuantileSampler, ready to ingest spans
func NewResourceQuantileSampler(conf *config.AgentConfig) *ResourceQuantileSampler {
	return &ResourceQuantileSampler{
		stats:         model.NewStatsBucket(0, 1, conf.LatencyResolution),
		traceBySpanID: map[uint64]*model.Trace{},
		conf:          conf,
	}
}

// AddTrace adds a span to the ResourceQuantileSampler internal memory
func (s *ResourceQuantileSampler) AddTrace(trace model.Trace) {
	s.mu.Lock()

	for _, span := range trace {
		s.traceBySpanID[span.SpanID] = &trace
		s.stats.HandleSpan(span, DefaultAggregators)
	}

	s.mu.Unlock()
}

// Flush returns representative spans based on GetSamples and reset its internal memory
func (s *ResourceQuantileSampler) Flush() []model.Trace {
	s.mu.Lock()
	traceBySpanID := s.traceBySpanID
	stats := s.stats

	s.traceBySpanID = map[uint64]*model.Trace{}
	s.stats = model.NewStatsBucket(0, 1, s.conf.LatencyResolution)
	s.mu.Unlock()

	return s.GetSamples(traceBySpanID, stats)
}

// GetSamples returns interesting spans by picking a representative of each SamplerQuantiles of each resource
func (s *ResourceQuantileSampler) GetSamples(
	traceBySpanID map[uint64]*model.Trace, stats model.StatsBucket,
) []model.Trace {
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

	var sampledSpans int
	result := make([]model.Trace, 0, len(selected))
	for tptr := range selected {
		result = append(result, *tptr)
		sampledSpans += len(*tptr)
	}

	return result
}

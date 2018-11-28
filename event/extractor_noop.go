package event

import "github.com/DataDog/datadog-trace-agent/agent"

// noopExtractor is a no-op APM event extractor used when APM event extraction is disabled.
type noopExtractor struct{}

// NewNoopExtractor returns a new APM event extractor that does not extract any events.
func NewNoopExtractor() Extractor {
	return &noopExtractor{}
}

func (e *noopExtractor) Extract(_ *agent.WeightedSpan, _ agent.SamplingPriority) (*agent.Event, float64, bool) {
	return nil, 0, false
}

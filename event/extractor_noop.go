package event

import "github.com/DataDog/datadog-trace-agent/model"

// noopExtractor is a no-op APM event extractor used when APM event extraction is disabled.
type noopExtractor struct{}

// NewNoopExtractor returns a new APM event extractor that does not extract any events.
func NewNoopExtractor() Extractor {
	return &noopExtractor{}
}

func (e *noopExtractor) Extract(_ *model.WeightedSpan, _ model.SamplingPriority) (*model.Event, float64) {
	return nil, 0
}

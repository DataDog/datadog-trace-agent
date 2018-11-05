package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// metricBasedExtractor is an event extractor that decides whether to extract APM events from spans based on
// the value of the event extraction rate metric set on those spans.
type metricBasedExtractor struct{}

// NewMetricBasedExtractor returns an APM event extractor that decides whether to extract APM events from spans based on
// the value of the event extraction rate metric set on those span.
func NewMetricBasedExtractor() Extractor {
	return &metricBasedExtractor{}
}

// Extract decides whether to extract APM events from a span based on the value of the event extraction rate metric set
// on that span. If priority is 2 (manually sampled) and extraction rate is bigger than 0, then an event is always
// extracted.
func (e *metricBasedExtractor) Extract(s *model.WeightedSpan, priority model.SamplingPriority) (extract bool, rate float64) {
	extractionRate, ok := s.GetMetric(model.KeySamplingRateEventExtraction)

	if !ok {
		return false, RateNone
	}

	if extractionRate > 0 && priority >= model.PriorityUserKeep {
		// If the trace has been manually sampled, we keep all matching spans
		return true, 1
	}

	return sampler.SampleByRate(s.TraceID, extractionRate), extractionRate
}

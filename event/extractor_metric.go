package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
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
// on that span. If such a value exists, the extracted event is returned along with this rate. Otherwise, nil is
// returned.
//
// NOTE: If priority is UserKeep (manually sampled) any extraction rate bigger than 0 is upscaled to 1 to ensure no
// extraction sampling is done on this event.
func (e *metricBasedExtractor) Extract(s *model.WeightedSpan, priority model.SamplingPriority) (*model.Event, float64) {
	extractionRate, ok := s.GetEventExtractionRate()
	if !ok {
		return nil, 0
	}
	if extractionRate > 0 && priority >= model.PriorityUserKeep {
		// If the trace has been manually sampled, we keep all matching spans
		extractionRate = 1
	}
	return &model.Event{
		Priority: priority,
		Span:     s.Span,
	}, extractionRate
}

package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// fixedRateExtractor is an event extractor that decides whether to extract APM events from spans based on
// `(service name, operation name) => sampling rate` mappings.
type fixedRateExtractor struct {
	rateByServiceAndName map[string]map[string]float64
}

// NewFixedRateExtractor returns an APM event extractor that decides whether to extract APM events from spans following
// the provided extraction rates for a span's (service name, operation name) pair.
func NewFixedRateExtractor(rateByServiceAndName map[string]map[string]float64) Extractor {
	return &fixedRateExtractor{
		rateByServiceAndName: rateByServiceAndName,
	}
}

// Extract decides to extract an apm event from a span if its service and name have a corresponding extraction rate
// on the rateByServiceAndName map passed in the constructor. The extracted event is returned along with the associated
// extraction rate or nil if no extraction happened.
func (e *fixedRateExtractor) Extract(s *model.WeightedSpan, priority model.SamplingPriority) (*model.Event, float64) {
	operations, ok := e.rateByServiceAndName[s.Service]
	if !ok {
		return nil, 0
	}
	extractionRate, ok := operations[s.Name]
	if !ok {
		return nil, 0
	}
	if extractionRate > 0 && priority >= model.PriorityUserKeep {
		// If the span has been manually sampled, we always want to keep these events
		extractionRate = 1
	}
	return &model.Event{
		Span:     s.Span,
		Priority: priority,
	}, extractionRate
}

package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// legacyExtractor is an event extractor that decides whether to extract APM events from spans based on
// `serviceName => sampling rate` mappings.
type legacyExtractor struct {
	rateByService map[string]float64
}

// NewLegacyExtractor returns an APM event extractor that decides whether to extract APM events from spans following the
// specified extraction rates for a span's service.
func NewLegacyExtractor(rateByService map[string]float64) Extractor {
	return &legacyExtractor{
		rateByService: rateByService,
	}
}

// Extract decides to extract an apm event from the provided span if there's an extraction rate configured for that
// span's service. In this case the extracted event is returned along with the found extraction rate. If this rate
// doesn't exist or the provided span is not a top level one, then no extraction is done and nil is returned.
func (e *legacyExtractor) Extract(s *model.WeightedSpan, priority model.SamplingPriority) (*model.Event, float64) {
	if !s.TopLevel {
		return nil, 0
	}
	extractionRate, ok := e.rateByService[s.Service]
	if !ok {
		return nil, 0
	}
	return &model.Event{
		Span:     s.Span,
		Priority: priority,
	}, extractionRate
}

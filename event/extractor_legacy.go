package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// legacyExtractor is an event extractor that extracts APM events from traces based on `serviceName => sampling
// ratio` mappings.
type legacyExtractor struct {
	rateByService map[string]float64
}

// NewLegacyExtractor returns an APM event extractor that extracts APM events from a trace following the specified
// extraction rates for any spans matching a specific service.
func NewLegacyExtractor(rateByService map[string]float64) Extractor {
	return &legacyExtractor{
		rateByService: rateByService,
	}
}

// Extract extracts apm events from the trace and returns them as a slice.
func (s *legacyExtractor) Extract(t model.ProcessedTrace) []*model.APMEvent {
	var events []*model.APMEvent

	for _, span := range t.WeightedTrace {
		if extracted, rate := s.shouldExtractEvent(span); extracted {
			event := &model.APMEvent{
				Span:         span.Span,
				TraceSampled: t.Sampled,
			}
			event.SetExtractionSampleRate(rate)

			events = append(events, event)
		}
	}

	return events
}

func (s *legacyExtractor) shouldExtractEvent(span *model.WeightedSpan) (bool, float64) {
	if !span.TopLevel {
		return false, 0
	}

	if extractionRate, ok := s.rateByService[span.Service]; ok {
		if sampler.SampleByRate(span.TraceID, extractionRate) {
			return true, extractionRate
		}
	}

	return false, 0
}

package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// fixedRateExtractor is an event extractor that extracts APM events from traces based on
// `(service name, operation name) => sampling rate` mappings.
type fixedRateExtractor struct {
	rateByServiceAndName map[string]map[string]float64
}

// NewFixedRateExtractor returns an APM event extractor that extracts APM events from a trace following the provided
// extraction rates for any spans matching a (service name, operation name) pair.
func NewFixedRateExtractor(rateByServiceAndName map[string]map[string]float64) Extractor {
	return &fixedRateExtractor{
		rateByServiceAndName: rateByServiceAndName,
	}
}

// Extract extracts analyzed spans from the trace and returns them as a slice
func (s *fixedRateExtractor) Extract(t model.ProcessedTrace) []*model.APMEvent {
	var events []*model.APMEvent

	// Get the trace priority
	priority, hasPriority := t.GetSamplingPriority()

	for _, span := range t.WeightedTrace {
		if extract, rate := s.shouldExtractEvent(span, hasPriority, priority); extract {
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

func (s *fixedRateExtractor) shouldExtractEvent(span *model.WeightedSpan, hasPriority bool, priority int) (extract bool, rate float64) {
	if operations, ok := s.rateByServiceAndName[span.Service]; ok {
		if extractionRate, ok := operations[span.Name]; ok {
			// If the trace has been manually sampled, we keep all matching spans
			if hasPriority && priority >= 2 {
				return true, 1
			}

			// Else we apply whatever rate was configured
			sampled := sampler.SampleByRate(span.TraceID, extractionRate)

			return sampled, extractionRate
		}
	}
	return false, 0
}

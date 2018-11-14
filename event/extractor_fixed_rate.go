package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
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
// and the span's trace id is chosen based on said rate.
//
// If no rate is set for the service and name pair in the span, the returned rate is UnknownRate.
func (e *fixedRateExtractor) Extract(s *model.WeightedSpan, priority model.SamplingPriority) (extract bool, rate float64, decided bool) {
	operations, ok := e.rateByServiceAndName[s.Service]
	if !ok {
		return false, 0, false
	}
	extractionRate, ok := operations[s.Name]
	if !ok {
		return false, 0, false
	}
	if extractionRate > 0 && priority >= model.PriorityUserKeep {
		// If the span has been manually sampled, we always want to extract events.
		return true, 1, true
	}
	return sampler.SampleByRate(s.TraceID, extractionRate), extractionRate, true
}

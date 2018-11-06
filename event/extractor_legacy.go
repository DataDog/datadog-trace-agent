package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
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

// Extract decides whether to extract an apm event from the provided span based on a sampling rate on that span's
// service. If this rate doesn't exist or the provided span is not a top level one, then no decision is done and
// UnknownRate is returned.
func (e *legacyExtractor) Extract(s *model.WeightedSpan, priority model.SamplingPriority) (extract bool, rate float64) {
	if !s.TopLevel {
		return false, RateNone
	}
	extractionRate, ok := e.rateByService[s.Service]
	if !ok {
		return false, RateNone
	}
	return sampler.SampleByRate(s.TraceID, extractionRate), extractionRate
}

package agent

import (
	"github.com/DataDog/datadog-trace-agent/internal/pb"
	"github.com/DataDog/datadog-trace-agent/internal/sampler"
)

// Event is an event extracted from received traces and sent to Datadog's Trace Search functionality.
type Event struct {
	Span     *pb.Span
	Priority sampler.SamplingPriority
}

// GetClientSampleRate gets the rate at which the trace from which we extracted this event was sampled at the tracer.
// NOTE: This defaults to 1 if no rate is stored.
func (e *Event) GetClientTraceSampleRate() float64 {
	return sampler.GetClientRate(e.Span)
}

// SetClientTraceSampleRate sets the rate at which the trace from which we extracted this event was sampled at the tracer.
func (e *Event) SetClientTraceSampleRate(rate float64) {
	sampler.SetClientRate(e.Span, rate)
}

// GetPreSampleRate gets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
// NOTE: This defaults to 1 if no rate is stored.
func (e *Event) GetPreSampleRate() float64 {
	return sampler.GetPreSampleRate(e.Span)
}

// SetPreSampleRate sets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
func (e *Event) SetPreSampleRate(rate float64) {
	sampler.SetPreSampleRate(e.Span, rate)
}

// GetExtractionSampleRate gets the rate at which the trace from which we extracted this event was sampled at the tracer.
// NOTE: This defaults to 1 if no rate is stored.
func (e *Event) GetExtractionSampleRate() float64 {
	return sampler.GetEventExtractionRate(e.Span)
}

// SetExtractionSampleRate sets the rate at which the trace from which we extracted this event was sampled at the tracer.
func (e *Event) SetExtractionSampleRate(rate float64) {
	sampler.SetEventExtractionRate(e.Span, rate)
}

// GetMaxEPSSampleRate gets the rate at which this event was sampled by the max eps event sampler.
func (e *Event) GetMaxEPSSampleRate() float64 {
	return sampler.GetMaxEPSRate(e.Span)
}

// SetMaxEPSSampleRate sets the rate at which this event was sampled by the max eps event sampler.
func (e *Event) SetMaxEPSSampleRate(rate float64) {
	sampler.SetMaxEPSRate(e.Span, rate)
}

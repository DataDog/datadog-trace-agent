package model

const (
	KeySamplingRateClientTrace     = "_dd.v1.rate.iusr"
	KeySamplingRateEventExtraction = "_dd.v1.rate.extr"
	KeySamplingRatePreSampler      = "_dd.v1.rate.apre"
	KeySamplingRateEventSampler    = "_dd.v1.rate.alim"
)

// APMEvent is an event extracted from received traces and sent to Datadog's Trace Search functionality.
type APMEvent struct {
	Span         *Span
	TraceSampled bool
}

// GetClientTraceSampleRate gets the rate at which the trace from which we extracted this event was sampled at the tracer.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetClientTraceSampleRate() float64 {
	return e.Span.GetMetricDefault(KeySamplingRateClientTrace, 1.0)
}

// SetClientTraceSampleRate sets the rate at which the trace from which we extracted this event was sampled at the tracer.
func (e *APMEvent) SetClientTraceSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(KeySamplingRateClientTrace, rate)
	} else {
		delete(e.Span.Metrics, KeySamplingRateClientTrace)
	}
}

// GetExtractionSampleRate gets the rate at which the trace from which we extracted this event was sampled at the tracer.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetExtractionSampleRate() float64 {
	return e.Span.GetMetricDefault(KeySamplingRateEventExtraction, 1.0)
}

// SetExtractionSampleRate sets the rate at which the trace from which we extracted this event was sampled at the tracer.
func (e *APMEvent) SetExtractionSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(KeySamplingRateEventExtraction, rate)
	} else {
		delete(e.Span.Metrics, KeySamplingRateEventExtraction)
	}
}

// GetPreSamplerSampleRate gets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetPreSamplerSampleRate() float64 {
	return e.Span.GetMetricDefault(KeySamplingRatePreSampler, 1.0)
}

// SetPreSamplerSampleRate sets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
func (e *APMEvent) SetPreSamplerSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(KeySamplingRatePreSampler, rate)
	} else {
		delete(e.Span.Metrics, KeySamplingRatePreSampler)
	}
}

// GetEventSamplerSampleRate gets the rate at which this event was sampled by the event sampler.
func (e *APMEvent) GetEventSamplerSampleRate() float64 {
	return e.Span.GetMetricDefault(KeySamplingRateEventSampler, 1.0)
}

// SetEventSamplerSampleRate sets the rate at which this event was sampled by the event sampler.
func (e *APMEvent) SetEventSamplerSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(KeySamplingRateEventSampler, rate)
	} else {
		delete(e.Span.Metrics, KeySamplingRateEventSampler)
	}
}

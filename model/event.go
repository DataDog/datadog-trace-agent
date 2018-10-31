package model

const (
	keySamplingRateClientTrace  = "_dd.v1.rate.iusr"
	keySamplingRateExtraction   = "_dd.v1.rate.extr"
	keySamplingRatePreSampler   = "_dd.v1.rate.apre"
	keySamplingRateEventSampler = "_dd.v1.rate.alim"
)

// APMEvent is an event extracted from received traces and sent to Datadog's Trace Search functionality.
type APMEvent struct {
	Span         *Span
	TraceSampled bool
}

// GetClientTraceSampleRate gets the rate at which the trace from which we extracted this event was sampled at the tracer.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetClientTraceSampleRate() float64 {
	return e.Span.GetMetricDefault(keySamplingRateClientTrace, 1.0)
}

// SetClientTraceSampleRate sets the rate at which the trace from which we extracted this event was sampled at the tracer.
func (e *APMEvent) SetClientTraceSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(keySamplingRateClientTrace, rate)
	} else {
		delete(e.Span.Metrics, keySamplingRateClientTrace)
	}
}

// GetExtractionSampleRate gets the rate at which the trace from which we extracted this event was sampled at the tracer.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetExtractionSampleRate() float64 {
	return e.Span.GetMetricDefault(keySamplingRateExtraction, 1.0)
}

// SetExtractionSampleRate sets the rate at which the trace from which we extracted this event was sampled at the tracer.
func (e *APMEvent) SetExtractionSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(keySamplingRateExtraction, rate)
	} else {
		delete(e.Span.Metrics, keySamplingRateExtraction)
	}
}

// GetPreSamplerSampleRate gets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetPreSamplerSampleRate() float64 {
	return e.Span.GetMetricDefault(keySamplingRatePreSampler, 1.0)
}

// SetPreSamplerSampleRate sets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
func (e *APMEvent) SetPreSamplerSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(keySamplingRatePreSampler, rate)
	} else {
		delete(e.Span.Metrics, keySamplingRatePreSampler)
	}
}

// GetEventSamplerSampleRate gets the rate at which this event was sampled by the event sampler.
func (e *APMEvent) GetEventSamplerSampleRate() float64 {
	return e.Span.GetMetricDefault(keySamplingRateEventSampler, 1.0)
}

// SetEventSamplerSampleRate sets the rate at which this event was sampled by the event sampler.
func (e *APMEvent) SetEventSamplerSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(keySamplingRateEventSampler, rate)
	} else {
		delete(e.Span.Metrics, keySamplingRateEventSampler)
	}
}

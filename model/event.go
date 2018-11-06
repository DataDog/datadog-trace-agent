package model

const (
	// KeySamplingRateEventExtraction is the key of the metric storing the event extraction rate on an APM event.
	KeySamplingRateEventExtraction = "_dd1.sr.eausr"
	// KeySamplingRateEventSampler is the key of the metric storing the event sampler rate on an APM event.
	KeySamplingRateEventSampler = "_dd1.sr.eamax"
)

// APMEvent is an event extracted from received traces and sent to Datadog's Trace Search functionality.
type APMEvent struct {
	Span         *Span
	TraceSampled bool
}

// GetClientSampleRate gets the rate at which the trace from which we extracted this event was sampled at the tracer.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetClientTraceSampleRate() float64 {
	return e.Span.GetClientSampleRate()
}

// SetClientTraceSampleRate sets the rate at which the trace from which we extracted this event was sampled at the tracer.
func (e *APMEvent) SetClientTraceSampleRate(rate float64) {
	e.Span.SetClientTraceSampleRate(rate)
}

// GetPreSampleRate gets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetPreSampleRate() float64 {
	return e.Span.GetPreSampleRate()
}

// SetPreSampleRate sets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
func (e *APMEvent) SetPreSampleRate(rate float64) {
	e.Span.SetPreSampleRate(rate)
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
		// We assume missing value is 1 to save bandwidth (check getter).
		delete(e.Span.Metrics, KeySamplingRateEventExtraction)
	}
}

// GetEventSampleRate gets the rate at which this event was sampled by the event sampler.
func (e *APMEvent) GetEventSampleRate() float64 {
	return e.Span.GetMetricDefault(KeySamplingRateEventSampler, 1.0)
}

// SetEventSampleRate sets the rate at which this event was sampled by the event sampler.
func (e *APMEvent) SetEventSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(KeySamplingRateEventSampler, rate)
	} else {
		// We assume missing value is 1 to save bandwidth (check getter).
		delete(e.Span.Metrics, KeySamplingRateEventSampler)
	}
}

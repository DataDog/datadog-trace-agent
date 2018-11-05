package model

const (
	// KeySamplingRateClientTrace is the key of the metric storing the trace client sampling rate on an APM event.
	KeySamplingRateClientTrace = "_dd.v1.rate.iusr"
	// KeySamplingRateEventExtraction is the key of the metric storing the event extraction rate on an APM event.
	KeySamplingRateEventExtraction = "_dd.v1.rate.extr"
	// KeySamplingRatePreSampler is the key of the metric storing the trace pre sampler rate on an APM event.
	KeySamplingRatePreSampler = "_dd.v1.rate.apre"
	// KeySamplingRateEventSampler is the key of the metric storing the event sampler rate on an APM event.
	KeySamplingRateEventSampler = "_dd.v1.rate.alim"
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

// GetPreSampleRate gets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
// NOTE: This defaults to 1 if no rate is stored.
func (e *APMEvent) GetPreSampleRate() float64 {
	return e.Span.GetMetricDefault(KeySamplingRatePreSampler, 1.0)
}

// SetPreSampleRate sets the rate at which the trace from which we extracted this event was sampled by the
// agent's presampler.
func (e *APMEvent) SetPreSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(KeySamplingRatePreSampler, rate)
	} else {
		delete(e.Span.Metrics, KeySamplingRatePreSampler)
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
		delete(e.Span.Metrics, KeySamplingRateEventSampler)
	}
}

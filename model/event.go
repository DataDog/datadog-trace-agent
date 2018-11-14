package model

const (
	// KeySamplingRateEventExtraction is the key of the metric storing the event extraction rate on an APM event.
	KeySamplingRateEventExtraction = "_dd1.sr.eausr"
	// KeySamplingRateMaxEPSSampler is the key of the metric storing the max eps sampler rate on an APM event.
	KeySamplingRateMaxEPSSampler = "_dd1.sr.eamax"
)

// APMEvent is an event extracted from received traces and sent to Datadog's Trace Search functionality.
type APMEvent struct {
	Span     *Span
	Priority SamplingPriority
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

// GetMaxEPSSampleRate gets the rate at which this event was sampled by the max eps event sampler.
func (e *APMEvent) GetMaxEPSSampleRate() float64 {
	return e.Span.GetMetricDefault(KeySamplingRateMaxEPSSampler, 1.0)
}

// SetMaxEPSSampleRate sets the rate at which this event was sampled by the max eps event sampler.
func (e *APMEvent) SetMaxEPSSampleRate(rate float64) {
	if rate < 1 {
		e.Span.SetMetric(KeySamplingRateMaxEPSSampler, rate)
	} else {
		// We assume missing value is 1 to save bandwidth (check getter).
		delete(e.Span.Metrics, KeySamplingRateMaxEPSSampler)
	}
}

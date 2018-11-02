package model

import (
	"math/rand"
)

const (
	// SpanSampleRateMetricKey is the metric key holding the sample rate
	SpanSampleRateMetricKey = "_sample_rate"
	// Fake type of span to indicate it is time to flush
	flushMarkerType = "_FLUSH_MARKER"

	// SamplingPriorityKey is the key of the sampling priority value in the metrics map of the root span
	SamplingPriorityKey = "_sampling_priority_v1"
)

// RandomID generates a random uint64 that we use for IDs
func RandomID() uint64 {
	return uint64(rand.Int63())
}

// IsFlushMarker tells if this is a marker span, which signals the system to flush
func (s *Span) IsFlushMarker() bool {
	return s.Type == flushMarkerType
}

// NewFlushMarker returns a new flush marker
func NewFlushMarker() *Span {
	return &Span{Type: flushMarkerType}
}

// End returns the end time of the span.
func (s *Span) End() int64 {
	return s.Start + s.Duration
}

// Weight returns the weight of the span as defined for sampling, i.e. the
// inverse of the sampling rate.
func (s *Span) Weight() float64 {
	if s == nil {
		return 1.0
	}
	sampleRate, ok := s.Metrics[SpanSampleRateMetricKey]
	if !ok || sampleRate <= 0.0 || sampleRate > 1.0 {
		return 1.0
	}

	return 1.0 / sampleRate
}

// GetMetric gets a value in the span Metrics map.
func (s *Span) GetMetric(k string) (float64, bool) {
	if s == nil || s.Metrics == nil {
		return 0, false
	}

	val, ok := s.Metrics[k]

	return val, ok
}

// GetMetricDefault gets a value in the span Metrics map or default if no value is stored there.
func (s *Span) GetMetricDefault(k string, def float64) float64 {
	if val, ok := s.GetMetric(k); ok {
		return val
	}

	return def
}

// SetMetric sets a value in the span Metrics map.
func (s *Span) SetMetric(key string, val float64) {
	if s.Metrics == nil {
		s.Metrics = make(map[string]float64)
	}
	s.Metrics[key] = val
}

// GetSamplingPriority returns the value of the sampling priority metric set on this span and a boolean indicating if
// such a metric was actually found or not.
func (s *Span) GetSamplingPriority() (int, bool) {
	p, ok := s.GetMetric(SamplingPriorityKey)
	return int(p), ok
}

// SetSamplingPriority sets the sampling priority value on this span, overwriting any previously set value.
func (s *Span) SetSamplingPriority(priority int) {
	s.SetMetric(SamplingPriorityKey, float64(priority))
}

// GetEventExtractionRate returns the set APM event extraction rate for this span.
func (s *Span) GetEventExtractionRate() (float64, bool) {
	return s.GetMetric(KeySamplingRateEventExtraction)
}

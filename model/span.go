package model

import (
	"math"
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

// SamplingPriority is the type encoding a priority sampling decision.
type SamplingPriority int8

const (
	// PriorityNone is the value for SamplingPriority when no priority sampling decision could be found.
	PriorityNone SamplingPriority = math.MinInt8
	// PriorityUserDrop is the value set by a user to explicitly drop a trace.
	PriorityUserDrop SamplingPriority = -1
	// PriorityAutoDrop is the value set by a tracer to suggest dropping a trace.
	PriorityAutoDrop SamplingPriority = 0
	// PriorityAutoKeep is the value set by a tracer to suggest keeping a trace.
	PriorityAutoKeep SamplingPriority = 1
	// PriorityUserKeep is the value set by a user to explicitly keep a trace.
	PriorityUserKeep SamplingPriority = 2
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
func (s *Span) GetSamplingPriority() (SamplingPriority, bool) {
	p, ok := s.GetMetric(SamplingPriorityKey)
	return SamplingPriority(p), ok
}

// SetSamplingPriority sets the sampling priority value on this span, overwriting any previously set value.
func (s *Span) SetSamplingPriority(priority SamplingPriority) {
	s.SetMetric(SamplingPriorityKey, float64(priority))
}

// GetEventExtractionRate returns the set APM event extraction rate for this span.
func (s *Span) GetEventExtractionRate() (float64, bool) {
	return s.GetMetric(KeySamplingRateEventExtraction)
}

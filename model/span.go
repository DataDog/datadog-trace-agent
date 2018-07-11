package model

import (
	"math/rand"
)

const (
	// SpanSampleRateMetricKey is the metric key holding the sample rate
	SpanSampleRateMetricKey = "_sample_rate"
	// Fake type of span to indicate it is time to flush
	flushMarkerType = "_FLUSH_MARKER"
)

const (
	// SamplingPriorityKey specifies the key used to set the sampling priority metric.
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

package model

import (
	"bytes"
	"fmt"
	"math/rand"
)

const (
	// SpanSampleRateMetricKey is the metric key holding the sample rate
	SpanSampleRateMetricKey = "_sample_rate"
)

// Span is the common struct we use to represent a dapper-like span
type Span struct {
	// Mandatory
	// Service & Name together determine what software we are measuring
	Service  string `json:"service" msg:"service"`   // the software running (e.g. pylons)
	Name     string `json:"name" msg:"name"`         // the metric name aka. the thing we're measuring (e.g. pylons.render OR psycopg2.query)
	Resource string `json:"resource" msg:"resource"` // the natural key of what we measure (/index OR SELECT * FROM a WHERE id = ?)
	TraceID  uint64 `json:"trace_id" msg:"trace_id"` // ID that all spans in the same trace share
	SpanID   uint64 `json:"span_id" msg:"span_id"`   // unique ID given to any span
	Start    int64  `json:"start" msg:"start"`       // nanosecond epoch of span start
	Duration int64  `json:"duration" msg:"duration"` // in nanoseconds
	Error    int32  `json:"error" msg:"error"`       // error status of the span, 0 == OK

	// Optional
	Meta     map[string]string  `json:"meta" msg:"meta"`           // arbitrary tags/metadata
	Metrics  map[string]float64 `json:"metrics" msg:"metrics"`     // arbitrary metrics
	ParentID uint64             `json:"parent_id" msg:"parent_id"` // span ID of the span in which this one was created
	Type     string             `json:"type" msg:"type"`           // protocol associated with the span

	// Those are cached information, they are here not only for optimization,
	// but because the func which fill their values read
	// the Metrics map and causes map read/write concurrent accesses.
	weight   float64 // caches the result of Weight() called on the root span
	topLevel bool    // caches the result of TopLevel()
}

// String formats a Span struct to be displayed as a string
func (s Span) String() string {
	return fmt.Sprintf(
		"Span[t_id:%d,s_id:%d,p_id:%d,ser:%s,name:%s,res:%s]",
		s.TraceID,
		s.SpanID,
		s.ParentID,
		s.Service,
		s.Name,
		s.Resource,
	)
}

// RandomID generates a random uint64 that we use for IDs
func RandomID() uint64 {
	return uint64(rand.Int63())
}

const flushMarkerType = "_FLUSH_MARKER"

// IsFlushMarker tells if this is a marker span, which signals the system to flush
func (s *Span) IsFlushMarker() bool {
	return s.Type == flushMarkerType
}

// NewFlushMarker returns a new flush marker
func NewFlushMarker() Span {
	return Span{Type: flushMarkerType}
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

// Spans is a slice of span pointers
type Spans []*Span

func (spans Spans) String() string {
	var buf bytes.Buffer

	buf.WriteString("Spans{")

	for i, span := range spans {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%v", span)
	}

	buf.WriteByte('}')

	return buf.String()
}

// GoString returns a description of a slice of spans.
func (spans Spans) GoString() string {
	var buf bytes.Buffer

	buf.WriteString("Spans{")

	for i, span := range spans {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%#v", span)
	}

	buf.WriteByte('}')

	return buf.String()
}

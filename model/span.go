package model

import (
	"fmt"
	"math/rand"
)

// Span is the common struct we use to represent a dapper-like span
type Span struct {
	// Mandatory
	// Service & Name together determine what software we are measuring
	Service  string `json:"service"`  // the software running (e.g. pylons)
	Name     string `json:"name"`     // the metric name aka. the thing we're measuring (e.g. pylons.render OR psycopg2.query)
	Resource string `json:"resource"` // the natural key of what we measure (/index OR SELECT * FROM a WHERE id = ?)
	TraceID  uint64 `json:"trace_id"` // ID that all spans in the same trace share
	SpanID   uint64 `json:"span_id"`  // unique ID given to any span
	Start    int64  `json:"start"`    // nanosecond epoch of span start
	Duration int64  `json:"duration"` // in nanoseconds
	Error    int32  `json:"error"`    // error status of the span, 0 == OK

	// Optional
	Meta     map[string]string  `json:"meta"`      // arbitrary tags/metadata
	Metrics  map[string]float64 `json:"metrics"`   // arbitrary metrics
	ParentID uint64             `json:"parent_id"` // span ID of the span in which this one was created
	Type     string             `json:"type"`      // protocol associated with the span
	Weight   float64			`json:"weight,omitempty"` // Client-side sampling weight
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

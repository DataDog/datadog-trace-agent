package model

import (
	"errors"
	"fmt"
	"math/rand"
)

// Span is the common struct we use to represent a dapper-like span
type Span struct {
	// Mandatory
	// App & Service together determine what software we are measuring
	App      string `json:"app"`      // the user instance of the service running (e.g. dogweb)
	Service  string `json:"service"`  // the software running (e.g. pylons)
	Name     string `json:"name"`     // the metric name aka. the thing we're measuring (e.g. pylons.render OR psycopg2.query)
	Resource string `json:"resource"` // the natural key of what we measure (/index OR SELECT * FROM a WHERE id = ?)
	TraceID  uint64 `json:"trace_id"` // ID that all spans in the same trace share
	SpanID   uint64 `json:"span_id"`  // unique ID given to any span
	Start    int64  `json:"start"`    // nanosecond epoch of span start
	Duration int64  `json:"duration"` // in nanoseconds
	Error    int32  `json:"error"`    // error status of the span, 0 == OK

	// Optional
	Meta     map[string]string `json:"meta"`      // arbitrary tags/metadata
	Metrics  map[string]int64  `json:"metrics"`   // arbitrary metrics
	ParentID uint64            `json:"parent_id"` // span ID of the span in which this one was created
	Type     string            `json:"type"`      // protocol associated with the span
}

// String formats a Span struct to be displayed as a string
func (s Span) String() string {
	return fmt.Sprintf(
		"Span[tid:%d,sid:%d,app:%s,ser:%s,nam:%s,res:%s]",
		s.TraceID,
		s.SpanID,
		s.App,
		s.Service,
		s.Name,
		s.Resource,
	)
}

// Normalize makes sure a Span is properly initialized and encloses the minimum required info
func (s *Span) Normalize() error {
	// Mandatory data
	// Int63() generates a non-negative pseudo-random 63-bit integer
	if s.TraceID == 0 {
		s.TraceID = RandomID()
	}
	if s.SpanID == 0 {
		s.SpanID = RandomID()
	}
	if s.App == "" {
		return errors.New("span.normalize: `App` must be set in span")
	}
	if s.Service == "" {
		return errors.New("span.normalize: `Service` must be set in span")
	}
	if s.Name == "" {
		return errors.New("span.normalize: `Name` must be set in span")
	}
	if s.Resource == "" {
		return errors.New("span.normalize: `Resource` must be set in span")
	}
	// an Error - if not set - is 0 which is equivalent to a success status
	if s.Start == 0 {
		// NOTE[leo] this is probably ok, but we might want to be stricter and error?
		s.Start = Now()
	}
	// a Duration can be zero if it's an annotation...

	// Optional data, Meta & Metrics can be nil
	return nil
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

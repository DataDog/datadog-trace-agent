package model

import (
	"errors"
	"fmt"
	"math/rand"
)

// Span is the common struct we use to represent a dapper-like span
type Span struct {
	// Mandatory
	TraceID  uint64 `json:"trace_id"` // ID that all spans in the same trace share
	SpanID   uint64 `json:"span_id"`  // unique ID given to any span
	Service  string `json:"service"`  // the name of the high-level application generating this span
	Resource string `json:"resource"` // the natural key of what we measure
	Error    int32  `json:"error"`    // error status of the span, 0 == OK
	Start    int64  `json:"start"`    // nanosecond epoch of span start
	Duration int64  `json:"duration"` // in nanoseconds

	// Optional
	Type     string            `json:"type"`      // protocol associated with the span
	ParentID uint64            `json:"parent_id"` // span ID of the span in which this one was created
	Meta     map[string]string `json:"meta"`      // arbitrary tags/metadata
	Metrics  map[string]int64  `json:"metrics"`   // arbitrary metrics
}

// String formats a Span struct to be displayed as a string
func (s Span) String() string {
	return fmt.Sprintf(
		"Span[t_id=%d,s_id=%d,p_id=%d,s=%s,r=%s,e=%d,st=%d,d=%d,t=%s,meta=%v,metrics=%v]",
		s.TraceID,
		s.SpanID,
		s.ParentID,
		s.Service,
		s.Resource,
		s.Error,
		s.Start,
		s.Duration,
		s.Type,
		s.Meta,
		s.Metrics,
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
	if s.Service == "" {
		return errors.New("span.normalize: `service` must be set in span")
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

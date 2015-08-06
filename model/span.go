package model

import (
	"math/rand"
	"time"
)

// TID is a placeholder type for a Trace ID
type TID uint64

// SID is a placeholder type for a Span ID
type SID uint32

// Span is the common struct we use to represent a dapper-like span
//	* TraceID is an ID that all spans in the same trace share
//	* SpanID is a unique ID given to any span
//  * ParentID is the span ID of the parent span if any, otherwise zeroed
//	* Service represents the application name for which the span is originating
//	* Start is a float UNIX timestamp for the span starting time
//	* Duration is a float number of seconds for the length of the span
//  * SampleSize represents how many spans this span actually stands for, or is zeroed
//	* Meta is a flattened arbitrary metadata map
type Span struct {
	TraceID  TID `json:"trace_id"`
	SpanID   SID `json:"span_id"`
	ParentID SID `json:"parent_id"`

	Service  string `json:"service"`
	Resource string `json:"resource"`
	Type     string `json:"type"`

	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`

	SampleSize uint32 `json:"sample_size"`

	Meta map[string]string `json:"meta"`
}

// Normalize makes sure a Span is properly initialized and encloses the minimum required info
func (s *Span) Normalize() {
	if s.Start == 0 {
		s.Start = Now()
	}
	if s.SampleSize == 0 {
		s.SampleSize = 1
	}
	if s.Meta == nil {
		s.Meta = map[string]string{}
	}

	// Create a new Trace when there is no context for this span
	if s.TraceID == 0 {
		s.TraceID = NewTID()
		s.SpanID = NewSID()
	}
}

// FormatStart returns a nice string of the span start time
func (s *Span) FormatStart() string {
	secs := int64(s.Start)
	nsecs := int64((s.Start - float64(secs)) * 1e9)
	date := time.Unix(secs, nsecs)

	return date.Format(time.StampMilli)
}

// Now returns the current timestamp in our span-compliant format
func Now() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

// NewTID returns a new randomly generated TID
func NewTID() TID {
	return TID(rand.Int63())
}

// NewSID returns a new randomly generated SID
func NewSID() SID {
	return SID(rand.Uint32())
}

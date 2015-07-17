package main

import (
	"math/rand"
	"time"
)

type tID uint64
type sID uint32

type Span struct {
	TraceID  tID `json:"trace_id"`
	SpanID   sID `json:"span_id"`
	ParentID sID `json:"parent_id"`

	Service  string `json:"service"`
	Resource string `json:"resource"`
	Type     string `json:"type"`

	// Dates and duration are in s
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`

	SampleSize uint32 `json:"sample_size"`

	// Arbitrary metadata
	Meta map[string]string `json:"meta"`
}

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
		s.TraceID = NewtID()
		s.SpanID = NewsID()
	}
}

func (s *Span) FormatStart() string {
	secs := int64(s.Start)
	nsecs := int64((s.Start - float64(secs)) * 1e9)
	date := time.Unix(secs, nsecs)

	return date.Format(time.StampMilli)
}

func Now() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

func NewtID() tID {
	return tID(rand.Int63())
}

func NewsID() sID {
	return sID(rand.Uint32())
}

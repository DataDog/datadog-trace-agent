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

	// Dates and duration are in s
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Duration float64 `json:"duration"`
	Type     string  `json:"type"`

	// Arbitrary metadata
	Meta map[string]string `json:"meta"`
}

func (s *Span) Normalize() {
	if s.Start == 0 {
		s.Start = Now()
	}
	if s.Meta == nil {
		s.Meta = map[string]string{}
	}

	// Create a new Trace when there is no context
	if s.TraceID == 0 {
		s.TraceID = NewtID()
		s.SpanID = NewsID()
	}

	// Set both end and duration for dev convenience (should be done by the backend)
	if s.Duration == 0 && s.End != 0 {
		s.Duration = s.End - s.Start
	} else if s.Duration != 0 && s.End == 0 {
		s.End = s.Start + s.Duration
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

package model

// Trace is a collection of spans with the same trace ID
type Trace []Span

// NewTraceFlushMarker returns a trace with a single span as flush marker
func NewTraceFlushMarker() Trace {
	return []Span{NewFlushMarker()}
}

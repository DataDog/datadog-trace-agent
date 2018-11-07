package model

// APMEvent is an event extracted from received traces and sent to Datadog's Trace Search functionality.
type APMEvent struct {
	Span         *Span
	TraceSampled bool
}

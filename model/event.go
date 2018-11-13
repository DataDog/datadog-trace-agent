package model

const (
	KeySamplingRateEventExtraction = "_dd.v1.rate.extr"
)

// APMEvent is an event extracted from received traces and sent to Datadog's Trace Search functionality.
type APMEvent struct {
	Span         *Span
	TraceSampled bool
}

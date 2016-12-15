package model

// TracesFromSpans transforms a slice of spans into a slice of traces
// grouping them by trace IDs
// FIXME[1.x] this can be removed as we get pre-assembled traces from
// clients
func TracesFromSpans(spans []Span) []Trace {
	var traces []Trace
	byID := make(map[uint64][]Span)
	for _, s := range spans {
		byID[s.TraceID] = append(byID[s.TraceID], s)
	}
	for _, t := range byID {
		traces = append(traces, t)
	}

	return traces
}

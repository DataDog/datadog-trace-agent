package model

const (
	// TraceMetricsTagKey is a tag key which, if set to true,
	// ensures all statistics are computed for this span.
	// [FIXME] *not implemented yet*
	TraceMetricsTagKey = "datadog.trace_metrics"

	topLevelTag  = "_top_level"
	trueTagValue = "true"
)

// ComputeTopLevel updates all the spans top-level attribute.
//
// A span is considered top-level if:
// - it's a root span
// - its parent is unknown (other part of the code, distributed trace)
// - its parent belongs to another service (in that case it's a "local root"
//   being the highest ancestor of other spans belonging to this service and
//   attached to it).
func (t Trace) ComputeTopLevel() {
	// build a lookup map
	spanIDToIdx := make(map[uint64]int, len(t))
	for i, v := range t {
		spanIDToIdx[v.SpanID] = i
	}

	// iterate on each span and mark them as top-level if relevant
	for i, span := range t {
		if span.ParentID != 0 {
			if parentIdx, ok := spanIDToIdx[span.ParentID]; ok && t[parentIdx].Service == span.Service {
				continue
			}
		}
		t[i].setTopLevel(true)
	}
}

// setTopLevel sets the top-level attribute of the span.
func (s *Span) setTopLevel(topLevel bool) {
	if !topLevel {
		if s.Meta == nil {
			return
		}
		delete(s.Meta, topLevelTag)
		if len(s.Meta) == 0 {
			s.Meta = nil
		}
		return
	}
	if s.Meta == nil {
		s.Meta = make(map[string]string, 1)
	}
	s.Meta[topLevelTag] = trueTagValue
}

// TopLevel returns true if span is top-level.
func (s *Span) TopLevel() bool {
	return s.Meta[topLevelTag] == trueTagValue
}

// ForceMetrics returns true if statistics computation should be forced for this span.
func (s *Span) ForceMetrics() bool {
	return s.Meta[TraceMetricsTagKey] == trueTagValue
}

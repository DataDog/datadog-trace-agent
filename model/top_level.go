package model

const (
	// TraceMetricsKey is a tag key which, if set to true,
	// ensures all statistics are computed for this span.
	// [FIXME] *not implemented yet*
	TraceMetricsKey = "datadog.trace_metrics"

	// This is a special metric, it's 1 if the span is top-level, 0 if not.
	topLevelKey  = "_top_level"
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
	for i, span := range t {
		spanIDToIdx[span.SpanID] = i
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

// TopLevelSpans returns the list of all span ids which are marked as top-level.
// You need to call ComputeTopLevel before, else it returns an empty map.
func (t Trace) TopLevelSpans() map[uint64]struct{} {
	ids := make(map[uint64]struct{}, len(t)) // possibly too big, but not gigantic yet ensure only one alloc is done
	for _, span := range t {
		if span.TopLevel() {
			ids[span.SpanID] = struct{}{}
		}
	}
	return ids
}

// setTopLevel sets the top-level attribute of the span.
func (s *Span) setTopLevel(topLevel bool) {
	if !topLevel {
		if s.Metrics == nil {
			return
		}
		delete(s.Metrics, topLevelKey)
		if len(s.Metrics) == 0 {
			s.Metrics = nil
		}
		return
	}
	if s.Metrics == nil {
		s.Metrics = make(map[string]float64, 1)
	}
	s.Metrics[topLevelKey] = 1
}

// TopLevel returns true if span is top-level.
func (s *Span) TopLevel() bool {
	return s.Metrics[topLevelKey] == 1
}

// ForceMetrics returns true if statistics computation should be forced for this span.
func (s *Span) ForceMetrics() bool {
	return s.Meta[TraceMetricsKey] == trueTagValue
}

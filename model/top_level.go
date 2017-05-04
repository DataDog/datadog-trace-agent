package model

// ComputeTopLevel updates all the spans TopLevel field.
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
		if span.ParentID == 0 {
			// Root Span -> top-level
			t[i].TopLevel = true
			continue
		}
		parentIdx, ok := spanIDToIdx[span.ParentID]
		if !ok {
			// Unknown parent -> top-level
			t[i].TopLevel = true
			continue
		}
		if t[parentIdx].Service != span.Service {
			// Parent and self have different services -> top-level
			t[i].TopLevel = true
			continue
		}
	}
}

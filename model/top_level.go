package model

const (
	topLevelTag  = "_top_level"
	topLevelTrue = "true"
)

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
		if span.ParentID != 0 {
			parentIdx, ok := spanIDToIdx[span.ParentID]
			if ok && t[parentIdx].Service == span.Service {
				continue
			}
		}

		if span.Meta == nil {
			t[i].Meta = make(map[string]string, 1)
		}
		t[i].Meta[topLevelTag] = topLevelTrue
	}
}

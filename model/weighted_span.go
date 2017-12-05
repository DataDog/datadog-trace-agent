package model

// WeightedSpan extends Span to contain weights required by the Concentrator.
type WeightedSpan struct {
	Weight   float64 // Span weight. Similar to the trace root.Weight().
	TopLevel bool    // Is this span a service top-level or not. Similar to span.TopLevel().

	*Span
}

// WeightedTrace is a slice of WeightedSpan pointers.
type WeightedTrace []*WeightedSpan

// NewWeightedTrace returns a weighted trace, with coefficient required by the concentrator.
func NewWeightedTrace(trace Trace, root *Span) WeightedTrace {
	wt := make(WeightedTrace, len(trace))

	weight := root.Weight()

	for i := range trace {
		wt[i] = &WeightedSpan{
			Span:     trace[i],
			Weight:   weight,
			TopLevel: trace[i].TopLevel(),
		}
	}

	return wt
}

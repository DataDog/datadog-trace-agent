package quantile

import (
	"math"
	"math/rand"
)

// WeightedSliceSummary associates a weight to a slice summary.
type WeightedSliceSummary struct {
	Weight float64
	*SliceSummary
}

func probabilisticRound(g int, weight float64) int {
	// deterministic seed
	rand.Seed(7337)

	raw := weight * float64(g)
	decimal := raw - math.Floor(raw)
	limit := rand.Float64()

	iraw := int(raw)
	if limit > decimal {
		iraw++
	}

	return iraw
}

// WeighSummary applies a weight factor to a slice summary and return it as a
// new slice.
func WeighSummary(s *SliceSummary, weight float64) *SliceSummary {
	sw := NewSliceSummary()
	sw.Entries = make([]Entry, 0, len(s.Entries))

	gsum := 0
	for _, e := range s.Entries {
		newg := probabilisticRound(e.G, weight)
		// if an entry is down to 0 delete it
		if newg != 0 {
			sw.Entries = append(sw.Entries,
				Entry{V: e.V, G: newg, Delta: e.Delta},
			)
			gsum += newg
		}
	}

	sw.N = gsum
	return sw
}

// BySlicesWeighted BySlices() is the BySlices version but combines multiple
// weighted slice summaries before returning the histogram
func BySlicesWeighted(summaries ...WeightedSliceSummary) []SummarySlice {
	if len(summaries) == 0 {
		return []SummarySlice{}
	}

	mergeSummary := WeighSummary(summaries[0].SliceSummary, summaries[0].Weight)
	if len(summaries) > 1 {
		for _, s := range summaries[1:] {
			sw := WeighSummary(s.SliceSummary, s.Weight)
			mergeSummary.Merge(sw)
		}
	}

	return mergeSummary.BySlices()
}

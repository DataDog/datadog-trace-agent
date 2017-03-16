package quantile

type WeightedSliceSummary struct {
	Weight float64
	*SliceSummary
}

func weighSummary(s *SliceSummary, w float64) *SliceSummary {
	sw := s.Copy()
	for i := range sw.Entries {
		sw.Entries[i].G = int(w * float64(sw.Entries[i].G)) // +1 ?
	}

	sw.N = int(w * float64(sw.N))
	return sw
}

// BySlicesWeighted BySlices() is the BySlices version but combines multiple
// weighted slice summaries before returning the histogram
func BySlicesWeighted(summaries ...WeightedSliceSummary) []SummarySlice {
	if len(summaries) == 0 {
		return []SummarySlice{}
	}

	mergeSummary := weighSummary(summaries[0].SliceSummary, summaries[0].Weight)
	if len(summaries) > 1 {
		for _, s := range summaries[1:] {
			sw := weighSummary(s.SliceSummary, s.Weight)
			mergeSummary.Merge(sw)
		}
	}

	return mergeSummary.BySlices()
}

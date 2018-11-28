package agent

type ProcessedTrace struct {
	Trace         Trace
	WeightedTrace WeightedTrace
	Root          *Span
	Env           string
	Sublayers     map[*Span][]SublayerValue
	Sampled       bool
}

func (pt *ProcessedTrace) Weight() float64 {
	if pt.Root == nil {
		return 1.0
	}
	return pt.Root.Weight()
}

func (pt *ProcessedTrace) GetSamplingPriority() (SamplingPriority, bool) {
	return pt.Root.GetSamplingPriority()
}

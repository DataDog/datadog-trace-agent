package agent

import "github.com/DataDog/datadog-trace-agent/internal/pb"

type ProcessedTrace struct {
	Trace         pb.Trace
	WeightedTrace WeightedTrace
	Root          *pb.Span
	Env           string
	Sublayers     map[*pb.Span][]SublayerValue
	Sampled       bool
}

func (pt *ProcessedTrace) Weight() float64 {
	if pt.Root == nil {
		return 1.0
	}
	return pt.Root.Weight()
}

func (pt *ProcessedTrace) GetSamplingPriority() (pb.SamplingPriority, bool) {
	return pt.Root.GetSamplingPriority()
}

package event

import "github.com/DataDog/datadog-trace-agent/model"

// sampledTraceSampler is an event sampler that ensures that events for a sampled trace are sampled as well.
type sampledTraceSampler struct {
}

// NewSampledTraceSampler creates a new instance of a sampledTraceSampler.
func NewSampledTraceSampler() Sampler {
	return &sampledTraceSampler{}
}

// Sample samples the provided event (returns true) if the corresponding trace was sampled.
func (sts *sampledTraceSampler) Sample(event *model.APMEvent) SamplingDecision {
	if event.TraceSampled {
		return DecisionSample
	}

	return DecisionNone
}

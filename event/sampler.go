package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// SamplingDecision contains the result of a sampling step.
type SamplingDecision int8

const (
	// DecisionNone represents the result of a sampling step where no decision was made.
	DecisionNone SamplingDecision = iota
	// DecisionSimple represents the result of a sampling step where it was decided to sample an event.
	DecisionSample
	// DecisionDontSample represents the result of a sampling step where it was decided not to sample an event.
	DecisionDontSample
)

// Sampler samples APM events according to implementation-defined techniques.
type Sampler interface {
	// Sample decides whether to sample the provided event or not.
	Sample(event *model.APMEvent) SamplingDecision
}

// BatchSampler allows sampling a collection of APM events, returning only those that survived sampling.
type BatchSampler struct {
	sampler Sampler
}

// NewBatchSampler creates a new BatchSampler using the provided underlying sampler.
func NewBatchSampler(sampler Sampler) *BatchSampler {
	return &BatchSampler{
		sampler: sampler,
	}
}

// Sample takes a collection of events, makes a sampling decision for each event and returns a collection containing
// only those events that were sampled.
func (bs *BatchSampler) Sample(events []*model.APMEvent) []*model.APMEvent {
	result := make([]*model.APMEvent, 0, len(events))

	for _, event := range events {
		if bs.sampler.Sample(event) == DecisionSample {
			result = append(result, event)
		}
	}

	return result
}

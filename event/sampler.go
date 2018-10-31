package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// SamplingDecision contains the result of a sampling step.
type SamplingDecision int8

// Sampler samples APM events according to implementation-defined techniques.
type Sampler interface {
	// Start tells this sampler to bootstrap whatever it needs to answer `Sample` requests.
	Start()
	// Sample decides whether to sample the provided event or not.
	Sample(event *model.APMEvent) bool
	// Stop tells this sampler to stop anything that was bootstrapped in `Start`.
	Stop()
}

// BatchSampler allows sampling a collection of APM events, returning only those that survived sampling.
type BatchSampler struct {
	sampler Sampler
}

// NewBatchSampler creates a new BatchSampler using the provided underlying sampler and sampling the event
// slice in place.
func NewBatchSampler(sampler Sampler) *BatchSampler {
	return &BatchSampler{
		sampler: sampler,
	}
}

// Start starts the underlying sampler.
func (bs *BatchSampler) Start() {
	bs.sampler.Start()
}

// Stop stops the underlying sampler.
func (bs *BatchSampler) Stop() {
	bs.sampler.Start()
}

// Sample takes a slice of events, makes a sampling decision for each event and modifies the passed slice in place,
// keeping only those events that were sampled. The returned slice uses the same underlying array as the one passed
// as an argument but is scoped to the number of sampled events.
// WARNING: The slice passed as argument is invalidated and should not be used again.
func (bs *BatchSampler) Sample(events []*model.APMEvent) []*model.APMEvent {
	writeIndex := 0

	for _, event := range events {
		if event == nil {
			continue
		}

		if bs.sampler.Sample(event) {
			events[writeIndex] = event
			writeIndex++
		}
	}

	return events[:writeIndex]
}

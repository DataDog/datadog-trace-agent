// Package sampler contains all the logic of the agent-side trace sampling
//
// Currently implementation is based on the scoring of the "signature" of each trace
// Based on the score, we get a sample rate to apply to the given trace
//
// Current score implementation is super-simple, it is a counter with polynomial decay per signature.
// We increment it for each incoming trace then we periodically divide the score by two every X seconds.
// Right after the division, the score is an approximation of the number of received signatures over X seconds.
// It is different from the scoring in the Agent.
package sampler

import (
	raclette "github.com/DataDog/raclette/model"
)

// Sampler is the main component of the sampling logic
type Sampler struct {
	// Storage of the state of the sampler
	backend *Backend

	// Extra sampling rate to combine to the existing sampling
	extraRate float64
}

// NewSampler returns an initialized Sampler
func NewSampler(extraRate float64) *Sampler {
	return &Sampler{
		backend:   NewBackend(),
		extraRate: extraRate,
	}
}

// Run runs and block on the Sampler main loop
func (s *Sampler) Run() {
	s.backend.Run()
}

// Stop stops the main Run loop
func (s *Sampler) Stop() {
	s.backend.Stop()
}

// Sample tells if the given trace is a sample which has to be kept
func (s *Sampler) Sample(trace raclette.Trace) bool {
	// Sanity check, just in case one trace is empty
	if len(trace) == 0 {
		return false
	}

	signature := ComputeSignature(trace)

	s.backend.CountSignature(signature)
	sampleRate := s.GetSampleRate(signature)

	// We should introduce pre-sampling trace-level normalization to be sure that's valid
	traceID := trace[0].TraceID

	return SampleByRate(traceID, sampleRate)
}

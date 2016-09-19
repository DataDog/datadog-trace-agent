// Package sampler contains all the logic of the agent-side trace sampling
//
// Currently implementation is based on the scoring of the "signature" of each trace
// Based on the score, we get a sample rate to apply to the given trace
//
// Current score implementation is super-simple, it is a counter with polynomial decay per signature.
// We increment it for each incoming trace then we periodically divide the score by two every X seconds.
// Right after the division, the score is an approximation of the number of received signatures over X seconds.
// It is different from the scoring in the Agent.
//
// Since the sampling can happen at different levels (client, agent, server) or depending on different rules,
// we have to track the sample rate applied at previous steps. This way, sampling twice at 50% can result in an
// effective 25% sampling. The rate is stored as a metric in the trace root.
package sampler

import (
	"math"
	"time"

	raclette "github.com/DataDog/raclette/model"
)

const (
	// SampleRateMetricKey is the metric key holding the sample rate
	SampleRateMetricKey = "_sample_rate"

	// Sampler parameters not (yet?) configurable
	defaultDecayPeriod          = 30 * time.Second
	defaultSignatureScoreOffset = float64(5)
	defaultSignatureScoreSlope  = float64(3)
)

// Sampler is the main component of the sampling logic
type Sampler struct {
	// Storage of the state of the sampler
	backend *Backend

	// Extra sampling rate to combine to the existing sampling
	extraRate float64
	// Maximum limit to the total number of traces per second to sample
	maxTPS float64

	// Sample any signature with a score lower than scoreSamplingOffset
	// It is basically the number of similar traces per second after which we start sampling
	signatureScoreOffset float64
	// Logarithm slope for the scoring function
	signatureScoreSlope float64
	// signatureScoreCoefficient = math.Pow(signatureScoreSlope, math.Log10(scoreSamplingOffset))
	signatureScoreCoefficient float64
}

// NewSampler returns an initialized Sampler
func NewSampler(extraRate float64, maxTPS float64) *Sampler {
	decayPeriod := defaultDecayPeriod
	signatureScoreOffset := defaultSignatureScoreOffset
	signatureScoreSlope := defaultSignatureScoreSlope

	return &Sampler{
		backend:   NewBackend(decayPeriod),
		extraRate: extraRate,
		maxTPS:    maxTPS,

		signatureScoreOffset:      signatureScoreOffset,
		signatureScoreSlope:       signatureScoreSlope,
		signatureScoreCoefficient: math.Pow(signatureScoreSlope, math.Log10(signatureScoreOffset)),
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

// Sample counts an incoming trace and tells if it is a sample which has to be kept
func (s *Sampler) Sample(trace raclette.Trace) bool {
	// Extra safety, just in case one trace is empty
	if len(trace) == 0 {
		return false
	}

	// We need the root in multiple steps of the sampling, so let's extract here
	// TODO: update raclette.Trace to contain a reference to the root, and don't give it as further function argument
	root := GetRoot(trace)
	signature := ComputeSignatureWithRoot(trace, root)

	// Update sampler state by counting this trace
	s.backend.CountSignature(signature)

	sampleRate := s.GetSampleRate(trace, root, signature)

	sampled := ApplySampleRate(root, sampleRate)

	if sampled {
		// Count the trace to allow us to check for the maxTPS limit.
		// It has to happen before the maxTPS sampling.
		s.backend.CountSample()

		// Check for the maxTPS limit, and if we require an extra sampling.
		// No need to check if we already decided not to keep the trace.
		maxTPSrate := s.GetMaxTPSSampleRate()
		if maxTPSrate < 1 {
			sampled = ApplySampleRate(root, maxTPSrate)
		}
	}

	return sampled
}

// GetSampleRate returns the sample rate to apply to a trace.
func (s *Sampler) GetSampleRate(trace raclette.Trace, root *raclette.Span, signature Signature) float64 {
	sampleRate := s.GetSignatureSampleRate(signature) * s.extraRate

	return sampleRate
}

// GetMaxTPSSampleRate returns an extra sample rate to apply if we are above maxTPS.
func (s *Sampler) GetMaxTPSSampleRate() float64 {
	// When above maxTPS, apply an additional sample rate to statistically respect the limit
	maxTPSrate := 1.0
	if s.maxTPS > 0 {
		// Overestimate the real score with the high limit of the backend bias.
		currentTPS := s.backend.GetSampledScore() * s.backend.decayFactor
		if currentTPS > s.maxTPS {
			maxTPSrate = s.maxTPS / currentTPS
		}
	}

	return maxTPSrate
}

// ApplySampleRate applies a sample rate over a trace root, returning if the trace should be sampled or not.
// It takes into account any previous sampling.
func ApplySampleRate(root *raclette.Span, sampleRate float64) bool {
	initialRate := GetTraceAppliedSampleRate(root)
	newRate := initialRate * sampleRate
	SetTraceAppliedSampleRate(root, newRate)

	traceID := root.TraceID

	return SampleByRate(traceID, newRate)
}

// GetTraceAppliedSampleRate gets the sample rate the sample rate applied earlier in the pipeline.
func GetTraceAppliedSampleRate(root *raclette.Span) float64 {
	if rate, ok := root.Metrics[SampleRateMetricKey]; ok {
		return rate
	}

	return 1.0
}

// SetTraceAppliedSampleRate sets the currently applied sample rate in the trace data to allow chained up sampling.
func SetTraceAppliedSampleRate(root *raclette.Span, sampleRate float64) {
	if root.Metrics == nil {
		root.Metrics = make(map[string]float64)
	}
	if _, ok := root.Metrics[SampleRateMetricKey]; !ok {
		root.Metrics[SampleRateMetricKey] = 1.0
	}
	root.Metrics[SampleRateMetricKey] = sampleRate
}

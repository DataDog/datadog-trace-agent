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
	// Metric key holding the sample rate
	SampleRateMetricKey = "_sample_rate"
)

// Sampler is the main component of the sampling logic
type Sampler struct {
	// Storage of the state of the sampler
	backend *Backend

	// Extra sampling rate to combine to the existing sampling
	extraRate float64

	// Sample any signature with a score lower than `scoreSamplingOffset`
	// It is basically the number of similar traces per second after which we start sampling
	signatureScoreOffset float64
	// Logarithm slope for the scoring function
	signatureScoreSlope float64
	// signatureScoreCoefficient = math.Pow(signatureScoreSlope, math.Log10(scoreSamplingOffset))
	signatureScoreCoefficient float64
}

// NewSampler returns an initialized Sampler
func NewSampler(extraRate float64) *Sampler {
	decayPeriod := 30 * time.Second

	signatureScoreOffset := float64(5)
	signatureScoreSlope := float64(3)

	return &Sampler{
		backend:   NewBackend(decayPeriod),
		extraRate: extraRate,

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

	SetTraceSampleRate(root, sampleRate)

	traceID := root.TraceID

	return SampleByRate(traceID, sampleRate)
}

// GetSampleRate returns the sample rate to apply to a trace, combining all possible mechanisms
func (s *Sampler) GetSampleRate(trace raclette.Trace, root *raclette.Span, signature Signature) float64 {
	sampleRate := s.GetSignatureSampleRate(signature) * GetTraceSampleRate(root) * s.extraRate

	return sampleRate
}

// GetTraceSampleRate gets the sample rate the sample rate applied earlier in the pipeline
func GetTraceSampleRate(root *raclette.Span) float64 {
	if rate, ok := root.Metrics[SampleRateMetricKey]; ok {
		return rate
	}

	return 1.0
}

// SetTraceSampleRate sets the currently applied sample rate in the trace data to allow chained up sampling
func SetTraceSampleRate(root *raclette.Span, sampleRate float64) {
	if root.Metrics == nil {
		root.Metrics = make(map[string]float64)
	}
	if _, ok := root.Metrics[SampleRateMetricKey]; !ok {
		root.Metrics[SampleRateMetricKey] = 1.0
	}
	root.Metrics[SampleRateMetricKey] = sampleRate
}

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

	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	// SampleRateMetricKey is the metric key holding the sample rate
	SampleRateMetricKey = "_sample_rate"

	// Sampler parameters not (yet?) configurable
	defaultDecayPeriod          time.Duration = 5 * time.Second
	initialSignatureScoreOffset float64       = 1
	minSignatureScoreOffset     float64       = 0.01
	defaultSignatureScoreSlope  float64       = 3
)

// Sampler is the main component of the sampling logic
type Sampler struct {
	// Storage of the state of the sampler
	Backend *Backend

	// Extra sampling rate to combine to the existing sampling
	extraRate float64
	// Maximum limit to the total number of traces per second to sample
	maxTPS float64

	// Sample any signature with a score lower than scoreSamplingOffset
	// It is basically the number of similar traces per second after which we start sampling
	signatureScoreOffset float64
	// Logarithm slope for the scoring function
	signatureScoreSlope float64
	// signatureScoreFactor = math.Pow(signatureScoreSlope, math.Log10(scoreSamplingOffset))
	signatureScoreFactor float64

	exit chan struct{}
}

// NewSampler returns an initialized Sampler
func NewSampler(extraRate float64, maxTPS float64) *Sampler {
	decayPeriod := defaultDecayPeriod

	s := &Sampler{
		Backend:   NewBackend(decayPeriod),
		extraRate: extraRate,
		maxTPS:    maxTPS,

		exit: make(chan struct{}),
	}

	s.SetSignatureCoefficients(initialSignatureScoreOffset, defaultSignatureScoreSlope)

	return s
}

// SetSignatureCoefficients updates the internal scoring coefficients used by the signature scoring
func (s *Sampler) SetSignatureCoefficients(offset float64, slope float64) {
	s.signatureScoreOffset = offset
	s.signatureScoreSlope = slope
	s.signatureScoreFactor = math.Pow(slope, math.Log10(offset))
}

// UpdateExtraRate updates the extra sample rate
func (s *Sampler) UpdateExtraRate(extraRate float64) {
	s.extraRate = extraRate
}

// UpdateMaxTPS updates the max TPS limit
func (s *Sampler) UpdateMaxTPS(maxTPS float64) {
	s.maxTPS = maxTPS
}

// Run runs and block on the Sampler main loop
func (s *Sampler) Run() {
	go s.Backend.Run()
	s.RunAdjustScoring()
}

// Stop stops the main Run loop
func (s *Sampler) Stop() {
	s.Backend.Stop()
	close(s.exit)
}

// RunAdjustScoring is the sampler feedback loop to adjust the scoring coefficients
func (s *Sampler) RunAdjustScoring() {
	t := time.NewTicker(2 * s.Backend.decayPeriod)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			s.AdjustScoring()
		case <-s.exit:
			return
		}
	}
}

// AdjustScoring modifies sampler coefficients to fit better the `maxTPS` condition
func (s *Sampler) AdjustScoring() {
	// See how far we are from our maxTPS limit and make signature sampler harder/softer accordingly
	currentTPS := s.Backend.GetSampledScore()
	totalTPS := s.Backend.GetTotalScore()
	TPSratio := currentTPS / s.maxTPS
	offset := s.signatureScoreOffset
	cardinality := float64(s.Backend.GetCardinality())

	coefficient := 1.0

	if TPSratio > 1 {
		// If above, reduce the offset
		coefficient = 0.8
		// If we keep 3x too many traces, reduce the offset even more
		if TPSratio > 3 {
			coefficient = 0.5
		}
	} else if TPSratio < 0.8 {
		// If below, increase the offset
		// Don't do it if:
		//  - we already keep all traces (with a 1% margin because of stats imprecision)
		//  - offset above maxTPS
		if currentTPS < 0.99*totalTPS && s.signatureScoreOffset < s.maxTPS {
			coefficient = 1.1
			if TPSratio < 0.5 {
				coefficient = 1.3
			}
		}
	}

	newOffset := coefficient * offset

	// Safeguard to avoid too small offset (for guaranteed very-low volume sampling)
	if newOffset < minSignatureScoreOffset {
		newOffset = minSignatureScoreOffset
	}

	// Default slope value
	newSlope := defaultSignatureScoreSlope

	// TODO: explain why this formula
	if offset < totalTPS {
		newSlope = math.Pow(10, math.Log10(cardinality*totalTPS/s.maxTPS)/math.Log10(totalTPS/minSignatureScoreOffset))
		// That's the max value we should allow. When slope == 10, we basically keep only `offset` traces per signature
		if newSlope > 10 {
			newSlope = 10
		}
	}

	s.SetSignatureCoefficients(newOffset, newSlope)
}

// Sample counts an incoming trace and tells if it is a sample which has to be kept
func (s *Sampler) Sample(trace model.Trace, root *model.Span, env string) bool {
	// Extra safety, just in case one trace is empty
	if len(trace) == 0 {
		return false
	}

	signature := ComputeSignatureWithRootAndEnv(trace, root, env)

	// Update sampler state by counting this trace
	s.Backend.CountSignature(signature)

	sampleRate := s.GetSampleRate(trace, root, signature)

	sampled := ApplySampleRate(root, sampleRate)

	if sampled {
		// Count the trace to allow us to check for the maxTPS limit.
		// It has to happen before the maxTPS sampling.
		s.Backend.CountSample()

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
func (s *Sampler) GetSampleRate(trace model.Trace, root *model.Span, signature Signature) float64 {
	sampleRate := s.GetSignatureSampleRate(signature) * s.extraRate

	return sampleRate
}

// GetMaxTPSSampleRate returns an extra sample rate to apply if we are above maxTPS.
func (s *Sampler) GetMaxTPSSampleRate() float64 {
	// When above maxTPS, apply an additional sample rate to statistically respect the limit
	maxTPSrate := 1.0
	if s.maxTPS > 0 {
		currentTPS := s.Backend.GetUpperSampledScore()
		if currentTPS > s.maxTPS {
			maxTPSrate = s.maxTPS / currentTPS
		}
	}

	return maxTPSrate
}

// ApplySampleRate applies a sample rate over a trace root, returning if the trace should be sampled or not.
// It takes into account any previous sampling.
func ApplySampleRate(root *model.Span, sampleRate float64) bool {
	initialRate := GetTraceAppliedSampleRate(root)
	newRate := initialRate * sampleRate
	SetTraceAppliedSampleRate(root, newRate)

	traceID := root.TraceID

	return SampleByRate(traceID, newRate)
}

// GetTraceAppliedSampleRate gets the sample rate the sample rate applied earlier in the pipeline.
func GetTraceAppliedSampleRate(root *model.Span) float64 {
	if rate, ok := root.Metrics[SampleRateMetricKey]; ok {
		return rate
	}

	return 1.0
}

// SetTraceAppliedSampleRate sets the currently applied sample rate in the trace data to allow chained up sampling.
func SetTraceAppliedSampleRate(root *model.Span, sampleRate float64) {
	if root.Metrics == nil {
		root.Metrics = make(map[string]float64)
	}
	if _, ok := root.Metrics[SampleRateMetricKey]; !ok {
		root.Metrics[SampleRateMetricKey] = 1.0
	}
	root.Metrics[SampleRateMetricKey] = sampleRate
}

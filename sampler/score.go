package sampler

import (
	"math"
)

const (
	// 2^64 - 1
	maxTraceID      = ^uint64(0)
	maxTraceIDFloat = float64(maxTraceID)
	// Good number for Knuth hashing (large, prime, fit in int64 for languages without uint64)
	samplerHasher = uint64(1111111111111111111)
)

// SampleByRate tells if a trace (from its ID) with a given rate should be sampled
// Use Knuth multiplicative hashing to leverage imbalanced traceID generators
func SampleByRate(traceID uint64, sampleRate float64) bool {
	if sampleRate < 1 {
		return traceID*samplerHasher < uint64(sampleRate*maxTraceIDFloat)
	}
	return true
}

// GetSignatureSampleRate gives the sample rate to apply to any signature
// For now, only based on count score
func (s *Sampler) GetSignatureSampleRate(signature Signature) float64 {
	score := s.GetCountScore(signature)

	if score > 1 {
		score = 1.0
	}

	return score
}

// GetCountScore scores any signature based on its recent throughput
// The score value can be seeing as the sample rate if the count were the only factor
// Since other factors can intervene (such as extra global sampling), its value can be larger than 1
func (s *Sampler) GetCountScore(signature Signature) float64 {
	score := s.Backend.GetSignatureScore(signature)

	return s.signatureScoreFactor / math.Pow(s.signatureScoreSlope, math.Log10(score))
}

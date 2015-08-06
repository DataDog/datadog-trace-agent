package main

import (
	"math"
	"math/rand"
)

// DurationGenerator is a function that returns the duration in seconds of a span
type DurationGenerator func() float64

// GaussianDuration is a DurationGenerator using a Gaussian distribution of times (mean, stdDev), cutoffs implement a way to set bounds on durations
func GaussianDuration(mean float64, stdDev float64, leftCutoff float64, rightCutoff float64) float64 {
	sample := rand.NormFloat64()*stdDev + mean
	if leftCutoff != 0 && sample < leftCutoff {
		return leftCutoff
	}
	if rightCutoff != 0 && sample > rightCutoff {
		return rightCutoff
	}

	// a duration can never be negative
	return math.Max(sample, 0)
}

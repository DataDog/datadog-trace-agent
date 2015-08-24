package main

import "math/rand"

// DurationGenerator is a function that returns the duration in seconds of a span
type DurationGenerator func() int64

// GaussianDuration is a DurationGenerator using a Gaussian distribution of times (mean, stdDev), cutoffs implement a way to set bounds on durations
func GaussianDuration(mean float64, stdDev float64, leftCutoff int64, rightCutoff int64) int64 {
	sample := int64((rand.NormFloat64()*stdDev + mean) * 1e9)
	if leftCutoff != 0 && sample < leftCutoff {
		return leftCutoff
	}
	if rightCutoff != 0 && sample > rightCutoff {
		return rightCutoff
	}

	// a duration can never be negative
	if sample < 0 {
		return 0
	}
	return sample
}

package sampler

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getTestBackend() *Backend {
	decayPeriod := 5 * time.Second

	return NewBackend(decayPeriod)
}

func randomSignature() Signature {
	return Signature(rand.Int63())
}

func TestBasicNewBackend(t *testing.T) {
	assert := assert.New(t)

	backend := getTestBackend()

	sign := randomSignature()
	backend.CountSignature(sign)

	assert.True(backend.GetSignatureScore(sign) > 0)
	assert.Equal(backend.GetSignatureScore(randomSignature()), 0)
}

func TestCountScoreConvergence(t *testing.T) {
	// With a constant number of tracesPerPeriod, the backend score should converge to tracesPerPeriod
	// Test the convergence of both signature and total sampled counters
	backend := getTestBackend()

	sign := randomSignature()

	periods := 50
	tracesPerPeriod := 1000
	period := backend.decayPeriod

	for period := 0; period < periods; period++ {
		backend.DecayScore()
		for i := 0; i < tracesPerPeriod; i++ {
			backend.CountSignature(sign)
			backend.CountSample()
		}
	}

	assert.InEpsilon(t, backend.GetSignatureScore(sign), float64(tracesPerPeriod)/period.Seconds(), 0.01)
	assert.InEpsilon(t, backend.GetSampledScore(), float64(tracesPerPeriod)/period.Seconds(), 0.01)
}

func TestCountScoreOblivion(t *testing.T) {
	// After some time, past traces shouldn't impact the score
	assert := assert.New(t)
	backend := getTestBackend()

	sign := randomSignature()

	// Number of tracesPerPeriod in the initial phase
	tracesPerPeriod := 1000
	ticks := 50

	for period := 0; period < ticks; period++ {
		backend.DecayScore()
		for i := 0; i < tracesPerPeriod; i++ {
			backend.CountSignature(sign)
		}
	}

	// Second phase: we stop receiving this signature

	// How long to wait until score is >50% the initial score (TODO: make it function of the config)
	halfLifePeriods := 6
	// How long to wait until score is >1% the initial score
	oblivionPeriods := 40

	for period := 0; period < halfLifePeriods; period++ {
		backend.DecayScore()
	}

	assert.True(backend.GetSignatureScore(sign) < 0.5*float64(tracesPerPeriod))

	for period := 0; period < oblivionPeriods-halfLifePeriods; period++ {
		backend.DecayScore()
	}

	assert.True(backend.GetSignatureScore(sign) < 0.01*float64(tracesPerPeriod))
}

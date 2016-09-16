package sampler

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getTestBackend() *Backend {
	return NewBackend(1 * time.Second)
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
	// With a constant input flow of tracesPerTick, the backend score should converge to tracesPerTick
	backend := getTestBackend()

	sign := randomSignature()

	ticks := 50
	tracesPerTick := 1000

	for tick := 0; tick < ticks; tick++ {
		backend.DecayScore()
		for i := 0; i < tracesPerTick; i++ {
			backend.CountSignature(sign)
		}
	}

	assert.InEpsilon(t, backend.GetSignatureScore(sign), tracesPerTick, 0.01)
}

func TestCountScoreOblivion(t *testing.T) {
	// After some time, past traces shouldn't impact the score
	assert := assert.New(t)
	backend := getTestBackend()

	sign := randomSignature()

	// Number of tracesPerTick in the initial phase
	tracesPerTick := 1000
	ticks := 50

	for tick := 0; tick < ticks; tick++ {
		backend.DecayScore()
		for i := 0; i < tracesPerTick; i++ {
			backend.CountSignature(sign)
		}
	}

	// Second phase: we stop receiving this signature

	// How long to wait until score is >50% the spike score (TODO: make it function of the config)
	halfLifeTicks := 6
	// How long to wait until score is >1% the spike score
	oblivionTicks := 40

	for tick := 0; tick < halfLifeTicks; tick++ {
		backend.DecayScore()
	}

	assert.True(backend.GetSignatureScore(sign) < 0.5*float64(tracesPerTick))

	for tick := 0; tick < oblivionTicks-halfLifeTicks; tick++ {
		backend.DecayScore()
	}

	assert.True(backend.GetSignatureScore(sign) < 0.01*float64(tracesPerTick))
}

package sampler

import (
	"time"
)

// Backend storing any state required to run the sampling algorithms
// The current algorithms only rely on counters of recent signatures, which we implement
// with simple counters with polynomial decay
type Backend struct {
	// Score per signature
	scores map[Signature]float64

	// Every decayPeriod, decay the score
	// Lower value is more reactive, but forget quicker
	decayPeriod time.Duration
	// At every decay tick, how much we reduce/divide the score
	// Lower value is more reactive, but forget quicker
	decayFactor float64
	// Factor to apply to move from the score to the representing number of traces per second.
	// By definition of the decay formula: countScaleFactor = (decayFactor / (decayFactor - 1)) * decayPeriod
	// It also represents by how much a spike is "smoothed" (if we instantly receive N times the same signature,
	// its immediate count will be increased by N / countScaleFactor)
	countScaleFactor float64

	exit chan bool
}

// NewBackend returns an initialized Backend
func NewBackend(decayPeriod time.Duration) *Backend {
	// With this factor, any past trace counts for less than 50% after 6*decayPeriod and >1% after 39*decayPeriod
	// We can keep it hardcoded, but having `decayPeriod` configurable should be enough?
	decayFactor := 1.125 // 9/8

	return &Backend{
		scores:           make(map[Signature]float64),
		decayPeriod:      decayPeriod,
		decayFactor:      decayFactor,
		countScaleFactor: (decayFactor / (decayFactor - 1)) * float64(decayPeriod/time.Second),
		exit:             make(chan bool),
	}
}

// Run runs and block on the Sampler main loop
func (b *Backend) Run() {
	for {
		select {
		case <-time.Tick(b.decayPeriod):
			b.DecayScore()
		case <-b.exit:
			return
		}
	}
}

// Stop stops the main Run loop
func (b *Backend) Stop() {
	close(b.exit)
}

// CountSignature counts an incoming signature
func (b *Backend) CountSignature(signature Signature) {
	b.scores[signature]++
}

// GetSignatureScore returns the score (representing the rolling count) of a signature
// GetSignatureScore returns the score (representing the rolling count as traces per sconde) of a signature
func (b *Backend) GetSignatureScore(signature Signature) float64 {
	return b.scores[signature] / b.countScaleFactor
}

// DecayScore applies the decay to the rolling counters
func (b *Backend) DecayScore() {
	for sig := range b.scores {
		score := b.scores[sig]
		if score > 2 {
			b.scores[sig] /= b.decayFactor
		} else {
			// When the score is too small, we can optimize by simply dropping the entry
			delete(b.scores, sig)
		}
	}
}

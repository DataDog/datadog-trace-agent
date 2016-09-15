package sampler

import (
	"time"
)

const (
	// Every decayPeriod, decay the score
	decayPeriod = 30 * time.Second
	// At every decay tick, how much we reduce/divide the score
	decayFactor = 1.125 // 9/8
	// The count is represented by score / countScaleFactor where countScaleFactor = (decayFactor / decayFactor - 1)
	countScaleFactor = 9
	// With this factor, any past trace counts for less than 50% after 6 * decayPeriod and >1% after 39 * decayPeriod
	// TODO: expose these as configurable and easy-to-understand options
)

// Backend storing any state required to run the sampling algorithms
// The current algorithms only rely on counters of recent signatures, which we implement
// with simple counters with polynomial decay
type Backend struct {
	// Score per signature
	scores map[Signature]float64
	exit   chan bool
}

// NewBackend returns an initialized Backend
func NewBackend() *Backend {
	return &Backend{
		scores: make(map[Signature]float64),
		exit:   make(chan bool),
	}
}

// Run runs and block on the Sampler main loop
func (s *Backend) Run() {
	for {
		select {
		case <-time.Tick(decayPeriod):
			s.DecayScore()
		case <-s.exit:
			return
		}
	}
}

// Stop stops the main Run loop
func (s *Backend) Stop() {
	close(s.exit)
}

// CountSignature counts an incoming signature
func (b *Backend) CountSignature(signature Signature) {
	b.scores[signature]++
}

// GetSignatureScore returns the score (representing the rolling count) of a signature
func (b *Backend) GetSignatureScore(signature Signature) float64 {
	return b.scores[signature] / countScaleFactor
}

// DecayScore applies the decay to the rolling counters
func (b *Backend) DecayScore() {
	for sig := range b.scores {
		score := b.scores[sig]
		if score > 2 {
			b.scores[sig] /= decayFactor
		} else {
			// When the score is too small, we can optimize by simply dropping the entry
			// TODO: this threshold should be function of the backend configuration
			delete(b.scores, sig)
		}
	}
}

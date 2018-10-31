package event

import (
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// maxEPSSampler (Max Events Per Second Sampler) is an event sampler that samples provided events so as to try to ensure
// no more than a certain amount of events is sampled per second.
type maxEPSSampler struct {
	maxEPS      float64
	rateCounter rateCounter
}

// NewMaxEPSSampler creates a new instance of a maxEPSSampler with the provided maximum amount of events per second.
func NewMaxEPSSampler(maxEPS float64) Sampler {
	return newMaxEPSSampler(maxEPS, newSamplerBackendRateCounter())
}

func newMaxEPSSampler(maxEPS float64, rateCounter rateCounter) Sampler {
	return &maxEPSSampler{
		maxEPS:      maxEPS,
		rateCounter: rateCounter,
	}
}

// Start starts the underlying rate counter.
func (s *maxEPSSampler) Start() {
	s.rateCounter.Start()
}

// Stop stops the underlying rate counter.
func (s *maxEPSSampler) Stop() {
	s.rateCounter.Stop()
}

// Sample determines whether or not we should sample the provided event in order to ensure no more than maxEPS events
// are sampled every second.
func (s *maxEPSSampler) Sample(event *model.APMEvent) bool {
	if event == nil {
		return false
	}

	// Count that we saw a new event
	s.rateCounter.Count()

	// Events with sampled traces are always kept even if that means going a bit above max eps.
	if event.TraceSampled {
		event.SetEventSamplerSampleRate(1)
		return true
	}

	maxEPSRate := 1.0
	currentEPS := s.rateCounter.GetRate()

	if currentEPS > s.maxEPS {
		maxEPSRate = s.maxEPS / currentEPS
	}

	sampled := sampler.SampleByRate(event.Span.TraceID, maxEPSRate)

	if sampled {
		event.SetEventSamplerSampleRate(maxEPSRate)
	}

	return sampled
}

// rateCounter keeps track of different event rates.
type rateCounter interface {
	Start()
	Count()
	GetRate() float64
	Stop()
}

// samplerBackendRateCounter is a rateCounter backed by a sampler.Backend.
type samplerBackendRateCounter struct {
	backend sampler.Backend
}

// newSamplerBackendRateCounter creates a new samplerBackendRateCounter based on exponential decay counters.
func newSamplerBackendRateCounter() *samplerBackendRateCounter {
	return &samplerBackendRateCounter{
		// TODO: Allow these to be configurable or study better defaults based on intended target
		backend: sampler.NewMemoryBackend(1*time.Second, 1.125),
	}
}

// Start starts the decaying of the backend rate counter.
func (sb *samplerBackendRateCounter) Start() {
	go sb.backend.Run()
}

// Stop stops the decaying of the backend rate counter.
func (sb *samplerBackendRateCounter) Stop() {
	sb.backend.Stop()
}

// Count adds an event to the rate computation.
func (sb *samplerBackendRateCounter) Count() {
	sb.backend.CountSample()
}

// GetRate gets the current event rate.
func (sb *samplerBackendRateCounter) GetRate() float64 {
	return sb.backend.GetUpperSampledScore()
}

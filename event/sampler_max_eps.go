package event

import (
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// maxEPSSampler (Max Events Per Second Sampler) is an event sampler that samples provided events so as to ensure no
// more than a certain amount of events is sampled per second.
type maxEPSSampler struct {
	maxEPS      float64
	rateCounter RateCounter
}

// NewMaxEPSSampler creates a new instance of a maxEPSSampler with the provided maximum amount of events per second.
func NewMaxEPSSampler(maxEPS float64, rateCounter RateCounter) Sampler {
	return &maxEPSSampler{
		maxEPS:      maxEPS,
		rateCounter: rateCounter,
	}
}

// Sample determines whether or not we should sample the provided event in order to ensure no more than maxEPS events
// are sampled every second.
func (s *maxEPSSampler) Sample(event *model.APMEvent) SamplingDecision {
	maxEPSRate := 1.0
	s.rateCounter.Count()
	currentEPS := s.rateCounter.GetRate()

	if currentEPS > s.maxEPS {
		maxEPSRate = s.maxEPS / currentEPS
	}

	sampled := sampler.SampleByRate(event.Span.TraceID, maxEPSRate)
	// TODO: Set maxEPSRate on the event

	if sampled {
		return DecisionSample
	}

	return DecisionDontSample
}

// RateCounter keeps track of different event rates.
type RateCounter interface {
	Count()
	GetRate() float64
}

// SamplerBackendRateCounter is a RateCounter backed by a sampler.Backend.
type SamplerBackendRateCounter struct {
	backend sampler.Backend
}

// NewSamplerBackendRateCounter creates a new SamplerBackendRateCounter based on exponential decay counters.
func NewSamplerBackendRateCounter() *SamplerBackendRateCounter {
	return &SamplerBackendRateCounter{
		// TODO: Allow these to be configurable or study better defaults based on intended target
		backend: sampler.NewMemoryBackend(1*time.Second, 1.125),
	}
}

// Start starts the decaying of the backend rate counter.
func (sb *SamplerBackendRateCounter) Start() {
	go sb.backend.Run()
}

// Stop stops the decaying of the backend rate counter.
func (sb *SamplerBackendRateCounter) Stop() {
	sb.backend.Stop()
}

// Count adds an event to the rate computation.
func (sb *SamplerBackendRateCounter) Count() {
	sb.backend.CountSample()
}

// GetRate gets the current event rate.
func (sb *SamplerBackendRateCounter) GetRate() float64 {
	return sb.backend.GetUpperSampledScore()
}

// ReadOnlyRateCounter is a read-only view of a backing RateCounter.
type ReadOnlyRateCounter struct {
	rateCounter RateCounter
}

// NewReadOnlyRateCounter creates a new ReadOnlyRateCounter wrapping the provided rate counter.
func NewReadOnlyRateCounter(rateCounter RateCounter) *ReadOnlyRateCounter {
	return &ReadOnlyRateCounter{
		rateCounter: rateCounter,
	}
}

// Count is a no-op on a ReadOnlyRateCounter.
func (ro *ReadOnlyRateCounter) Count() {
	// no-op
}

// GetRate returns the current rate of the underlying rate counter.
func (ro *ReadOnlyRateCounter) GetRate() float64 {
	return ro.rateCounter.GetRate()
}

package main

import (
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	mu            sync.Mutex
	sampledTraces []model.Trace
	traceCount    int
	lastFlush     time.Time

	samplerEngine SamplerEngine
}

// samplerStats contains sampler statistics
type samplerStats struct {
	// KeptTPS is the number of traces kept (average per second for last flush)
	KeptTPS float64
	// TotalTPS is the total number of traces (average per second for last flush)
	TotalTPS float64
}

type samplerInfo struct {
	// Stats contains statistics about what the sampler is doing.
	Stats samplerStats
	// State is the internal state of the sampler (for debugging mostly)
	State sampler.InternalState
}

// SamplerEngine cares about telling if a trace is a proper sample or not
type SamplerEngine interface {
	Run()
	Stop()
	Sample(t model.Trace, root *model.Span, env string) bool
}

// NewSampler creates a new empty sampler ready to be started
func NewSampler(conf *config.AgentConfig) *Sampler {
	return &Sampler{
		sampledTraces: []model.Trace{},
		traceCount:    0,
		samplerEngine: sampler.NewSampler(conf.ExtraSampleRate, conf.MaxTPS),
	}
}

// Run starts sampling traces
func (s *Sampler) Run() {
	go s.samplerEngine.Run()
}

// Add samples a trace then keep it until the next flush
func (s *Sampler) Add(t processedTrace) {
	s.mu.Lock()
	s.traceCount++
	if s.samplerEngine.Sample(t.Trace, t.Root, t.Env) {
		s.sampledTraces = append(s.sampledTraces, t.Trace)
	}
	s.mu.Unlock()
}

// Stop stops the sampler
func (s *Sampler) Stop() {
	s.samplerEngine.Stop()
}

// Flush returns representative spans based on GetSamples and reset its internal memory
func (s *Sampler) Flush() []model.Trace {
	s.mu.Lock()

	traces := s.sampledTraces
	s.sampledTraces = []model.Trace{}
	traceCount := s.traceCount
	s.traceCount = 0

	now := time.Now()
	duration := now.Sub(s.lastFlush)
	s.lastFlush = now

	s.mu.Unlock()

	state := s.samplerEngine.(*sampler.Sampler).GetState()
	var stats samplerStats
	if duration > 0 {
		stats.KeptTPS = float64(len(traces)) * float64(time.Second) / float64(duration)
		stats.TotalTPS = float64(traceCount) * float64(time.Second) / float64(duration)
	}

	log.Debugf("flushed %d sampled traces out of %d", len(traces), traceCount)
	log.Debugf("inTPS: %f, outTPS: %f, maxTPS: %f, offset: %f, slope: %f, cardinality: %d",
		state.InTPS, state.OutTPS, state.MaxTPS, state.Offset, state.Slope, state.Cardinality)

	// publish through expvar
	updateSamplerInfo(samplerInfo{Stats: stats, State: state})

	return traces
}

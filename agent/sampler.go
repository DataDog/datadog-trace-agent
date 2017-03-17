package main

import (
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/statsd"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	mu            sync.Mutex
	sampledTraces []model.Trace
	traceCount    int

	samplerEngine SamplerEngine
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
	s.mu.Unlock()

	s.logState()
	statsd.Client.Count("datadog.trace_agent.sampler.trace.kept", int64(len(traces)), nil, 1)
	statsd.Client.Count("datadog.trace_agent.sampler.trace.total", int64(traceCount), nil, 1)
	log.Debugf("flushed %d sampled traces out of %v", len(traces), traceCount)

	return traces
}

// logState is a debug logging of the sampler internals, to check its adaptative behavior
// This is mainly for dev purpose to watch over the adaptative behavior
// TODO: remove (or clean it) in a real released build
func (s *Sampler) logState() {
	state := s.samplerEngine.(*sampler.Sampler).GetState()
	log.Debugf("inTPS: %f, outTPS: %f, maxTPS: %f, offset: %f, slope: %f, cardinality: %d",
		state.InTPS, state.OutTPS, state.MaxTPS, state.Offset, state.Slope, state.Cardinality)

	updateSamplerState(state) // publish through expvar

	statsd.Client.Gauge("datadog.trace_agent.sampler.scoring.offset", state.Offset, nil, 1)
	statsd.Client.Gauge("datadog.trace_agent.sampler.scoring.slope", state.Slope, nil, 1)
	statsd.Client.Gauge("datadog.trace_agent.sampler.scoring.cardinality", float64(state.Cardinality), nil, 1)
	statsd.Client.Gauge("datadog.trace_agent.sampler.scoring.in_tps", state.InTPS, nil, 1)
	statsd.Client.Gauge("datadog.trace_agent.sampler.scoring.out_tps", state.OutTPS, nil, 1)
	statsd.Client.Gauge("datadog.trace_agent.sampler.scoring.max_tps", state.MaxTPS, nil, 1)
}

package main

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/statsd"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	sampledTraces []model.Trace

	// statistics
	traceCount int

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
	s.traceCount++
	if s.samplerEngine.Sample(t.Trace, t.Root, t.Env) {
		s.sampledTraces = append(s.sampledTraces, t.Trace)
	}
}

// Stop stops the sampler
func (s *Sampler) Stop() {
	s.samplerEngine.Stop()
}

// Flush returns representative spans based on GetSamples and reset its internal memory
func (s *Sampler) Flush() []model.Trace {
	traces := s.sampledTraces
	s.sampledTraces = []model.Trace{}
	traceCount := s.traceCount
	s.traceCount = 0

	statsd.Client.Count("trace_agent.sampler.trace.kept", int64(len(traces)), nil, 1)
	statsd.Client.Count("trace_agent.sampler.trace.total", int64(traceCount), nil, 1)
	log.Debugf("flushed %d sampled traces out of %v", len(traces), traceCount)

	return traces
}

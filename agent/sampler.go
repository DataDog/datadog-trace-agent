package main

import (
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/sampler"
	"github.com/DataDog/raclette/statsd"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	in  chan model.Trace
	out chan []model.Trace

	sampledTraces []model.Trace
	mu            sync.Mutex

	// statistics
	traceCount int

	se SamplerEngine
}

// SamplerEngine cares about telling if a trace is a proper sample or not
type SamplerEngine interface {
	Run()
	Stop()
	Sample(t model.Trace) bool
}

// NewSampler creates a new empty sampler ready to be started
func NewSampler(in chan model.Trace, conf *config.AgentConfig) *Sampler {
	return &Sampler{
		in:            in,
		out:           make(chan []model.Trace),
		sampledTraces: []model.Trace{},
		traceCount:    0,
		se:            sampler.NewSampler(conf.ExtraSampleRate, conf.MaxTPS),
	}
}

// Run starts sampling traces
func (s *Sampler) Run() {
	statsdTags := []string{"sampler:signature"}

	go s.se.Run()

	for trace := range s.in {
		if len(trace) == 1 && trace[0].IsFlushMarker() {
			traces := s.Flush()
			statsd.Client.Count("trace_agent.sampler.trace.kept", int64(len(traces)), statsdTags, 1)
			statsd.Client.Count("trace_agent.sampler.trace.total", int64(s.traceCount), statsdTags, 1)
			log.Debugf("flushed %d sampled traces out of %v", len(traces), s.traceCount)

			s.traceCount = 0
			s.out <- traces
		} else {
			s.AddTrace(trace)
			s.traceCount++
		}
	}

	close(s.out)
	s.se.Stop()
}

// AddTrace samples a trace then keep it until the next flush
func (s *Sampler) AddTrace(trace model.Trace) {
	if s.se.Sample(trace) {
		s.mu.Lock()
		s.sampledTraces = append(s.sampledTraces, trace)
		s.mu.Unlock()
	}
}

// Flush returns representative spans based on GetSamples and reset its internal memory
func (s *Sampler) Flush() []model.Trace {
	s.mu.Lock()
	samples := s.sampledTraces
	s.sampledTraces = []model.Trace{}
	s.mu.Unlock()

	return samples
}

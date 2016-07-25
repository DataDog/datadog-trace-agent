package main

import (
	"time"

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

	// statistics
	traceCount int

	se SamplerEngine
}

// SamplerEngine cares about ingesting spans and stats to return a sampled payload
type SamplerEngine interface {
	AddTrace(t model.Trace)
	Flush() []model.Trace
}

// NewSampler creates a new empty sampler ready to be started
func NewSampler(in chan model.Trace, conf *config.AgentConfig) *Sampler {
	return &Sampler{
		in:         in,
		out:        make(chan []model.Trace),
		traceCount: 0,
		se:         sampler.NewSignatureSampler(conf.ScoreThreshold, conf.SignaturePeriod, conf.ScoreJitter, conf.TPSMax),
	}
}

// Run starts sampling traces
func (s *Sampler) Run() {
	statsdTags := []string{"sampler:signature"}

	for trace := range s.in {
		if len(trace) == 1 && trace[0].IsFlushMarker() {
			startTime := time.Now()
			traces := s.se.Flush()
			execTime := time.Since(startTime)
			statsd.Client.Gauge("trace_agent.sampler.sample_duration", execTime.Seconds(), statsdTags, 1)

			statsd.Client.Count("trace_agent.sampler.trace.kept", int64(len(traces)), statsdTags, 1)
			statsd.Client.Count("trace_agent.sampler.trace.total", int64(s.traceCount), statsdTags, 1)
			log.Debugf("flushed %d sampled traces out of %v", len(traces), s.traceCount)

			s.traceCount = 0
			s.out <- traces
		} else {
			s.se.AddTrace(trace)
			s.traceCount++
		}
	}

	close(s.out)
}

package main

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/sampler"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	in  chan model.Trace
	out chan []model.Trace

	conf *config.AgentConfig

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
		in:   in,
		out:  make(chan []model.Trace),
		conf: conf,
		se:   sampler.NewSignatureSampler(conf),
	}
}

// Run starts sampling traces
func (s *Sampler) Run() {
	for trace := range s.in {
		if len(trace) == 1 && trace[0].IsFlushMarker() {
			traces := s.se.Flush()
			log.Debugf("sampler flushed %d traces", len(traces))
			s.out <- traces
		} else {
			s.se.AddTrace(trace)
		}
	}

	close(s.out)
}

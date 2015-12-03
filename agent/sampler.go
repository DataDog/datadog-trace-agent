package main

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/sampler"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	in  chan model.Span
	out chan []model.Span

	conf *config.AgentConfig

	se SamplerEngine

	Worker
}

// SamplerEngine cares about ingesting spans and stats to return a sampled payload
type SamplerEngine interface {
	AddSpan(span model.Span)
	Flush() []model.Span
}

// NewSampler creates a new empty sampler ready to be started
func NewSampler(in chan model.Span, conf *config.AgentConfig) *Sampler {
	s := &Sampler{
		in:   in,
		out:  make(chan []model.Span),
		conf: conf,
		se:   sampler.NewResourceQuantileSampler(conf),
	}
	s.Init()
	return s
}

// Start runs the Sampler by sending incoming spans to the SamplerEngine and flushing it on demand
func (s *Sampler) Start() {
	go s.run()
	log.Info("Sampler started")
}

func (s *Sampler) run() {
	s.wg.Add(1)
	for {
		select {
		case span := <-s.in:
			if span.IsFlushMarker() {
				log.Debug("Sampler starts a flush")
				spans := s.se.Flush()
				log.Debugf("Sampler flushes %d spans", len(spans))
				s.out <- spans
			} else {
				s.se.AddSpan(span)
			}
		case <-s.exit:
			log.Info("Sampler exiting")
			close(s.out)
			s.wg.Done()
			return
		}
	}
}

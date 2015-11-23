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

// NewSampler creates a new empty sampler
func NewSampler(
	in chan model.Span, conf *config.AgentConfig,
) *Sampler {
	s := &Sampler{
		in:  in,
		out: make(chan []model.Span),

		conf: conf,

		se: sampler.NewResourceQuantileSampler(conf),
	}
	s.Init()
	return s
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (s *Sampler) Start() {
	s.wg.Add(1)
	go s.run()

	log.Info("Sampler started")
}

// We rely on the concentrator ticker to flush periodically traces "aligning" on the buckets
// (it's not perfect, but we don't really care, traces of this stats bucket may arrive in the next flush)
func (s *Sampler) run() {
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

package main

import (
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/sampler"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	inSpans chan model.Span
	inStats chan model.StatsBucket  // Trigger the flush of the sampler when stats are received
	out     chan model.AgentPayload // Output the stats + samples

	conf *config.AgentConfig

	se SamplerEngine

	// exit channels used for synchronisation and sending stop signals
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// SamplerEngine cares about ingesting spans and stats to return a sampled payload
type SamplerEngine interface {
	AddSpan(span model.Span)
	FlushPayload(sb model.StatsBucket) model.AgentPayload
}

// NewSampler creates a new empty sampler
func NewSampler(
	inSpans chan model.Span, inStats chan model.StatsBucket, conf *config.AgentConfig, exit chan struct{}, exitGroup *sync.WaitGroup,
) *Sampler {

	return &Sampler{
		inSpans: inSpans,
		inStats: inStats,
		out:     make(chan model.AgentPayload),

		conf: conf,

		exit:      exit,
		exitGroup: exitGroup,

		se: sampler.NewResourceQuantileSampler(conf),
	}
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (s *Sampler) Start() {
	s.exitGroup.Add(1)
	go s.run()

	log.Info("Sampler started")
}

// We rely on the concentrator ticker to flush periodically traces "aligning" on the buckets
// (it's not perfect, but we don't really care, traces of this stats bucket may arrive in the next flush)
func (s *Sampler) run() {
	for {
		select {
		case span := <-s.inSpans:
			s.se.AddSpan(span)
		case bucket := <-s.inStats:
			log.Info("Received a bucket from concentrator, initiating a sampling+flush")
			s.out <- s.se.FlushPayload(bucket)
		case <-s.exit:
			log.Info("Sampler exiting")
			s.exitGroup.Done()
			return
		}
	}
}

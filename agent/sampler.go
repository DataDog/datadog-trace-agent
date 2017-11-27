package main

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	mu            sync.Mutex
	sampledTraces []model.Trace
	traceCount    int
	lastFlush     time.Time

	engine sampler.Engine
}

// samplerStats contains sampler statistics
type samplerStats struct {
	// KeptTPS is the number of traces kept (average per second for last flush)
	KeptTPS float64
	// TotalTPS is the total number of traces (average per second for last flush)
	TotalTPS float64
}

type samplerInfo struct {
	// EngineType contains the type of the engine (tells old sampler and new distributed sampler apart)
	EngineType string
	// Stats contains statistics about what the sampler is doing.
	Stats samplerStats
	// State is the internal state of the sampler (for debugging mostly)
	State sampler.InternalState
}

// NewScoreEngine creates a new empty sampler ready to be started
func NewScoreEngine(conf *config.AgentConfig) *Sampler {
	return &Sampler{
		sampledTraces: []model.Trace{},
		traceCount:    0,
		engine:        sampler.NewScoreEngine(conf.ExtraSampleRate, conf.MaxTPS),
	}
}

// NewPriorityEngine creates a new empty distributed sampler ready to be started
func NewPriorityEngine(conf *config.AgentConfig, dynConf *config.DynamicConfig) *Sampler {
	return &Sampler{
		sampledTraces: []model.Trace{},
		traceCount:    0,
		engine:        sampler.NewPriorityEngine(conf.ExtraSampleRate, conf.MaxTPS, &dynConf.RateByService),
	}
}

// Run starts sampling traces
func (s *Sampler) Run() {
	go func() {
		defer watchdog.LogOnPanic()
		s.engine.Run()
	}()
}

// Add samples a trace then keep it until the next flush
func (s *Sampler) Add(t processedTrace) (kept bool) {
	s.mu.Lock()
	s.traceCount++
	if kept = s.engine.Sample(t.Trace, t.Root, t.Env); kept {
		s.sampledTraces = append(s.sampledTraces, t.Trace)
	}
	s.mu.Unlock()

	return kept
}

// Stop stops the sampler
func (s *Sampler) Stop() {
	s.engine.Stop()
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

	state := s.engine.GetState()

	switch state := state.(type) {
	case sampler.InternalState:
		var stats samplerStats
		if duration > 0 {
			stats.KeptTPS = float64(len(traces)) / duration.Seconds()
			stats.TotalTPS = float64(traceCount) / duration.Seconds()
		}

		log.Debugf("flushed %d sampled traces out of %d", len(traces), traceCount)
		log.Debugf("inTPS: %f, outTPS: %f, maxTPS: %f, offset: %f, slope: %f, cardinality: %d",
			state.InTPS, state.OutTPS, state.MaxTPS, state.Offset, state.Slope, state.Cardinality)

		// publish through expvar
		switch s.engine.(type) {
		case *sampler.ScoreEngine:
			updateSamplerInfo(samplerInfo{EngineType: fmt.Sprint(reflect.TypeOf(s.engine)), Stats: stats, State: state})
		case *sampler.PriorityEngine:
			updatePrioritySamplerInfo(samplerInfo{EngineType: fmt.Sprint(reflect.TypeOf(s.engine)), Stats: stats, State: state})
		}
	default:
		log.Debugf("unhandled sampler engine, can't log state")
	}

	return traces
}

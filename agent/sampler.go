package main

import (
	"fmt"
	"reflect"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// Sampler chooses wich spans to write to the API
type Sampler struct {
	sampled chan *model.Trace

	// For stats
	keptTraceCount  int
	totalTraceCount int
	lastFlush       time.Time

	// actual implementation of the sampling logic
	engine sampler.Engine
}

// NewScoreSampler creates a new empty sampler ready to be started
func NewScoreSampler(conf *config.AgentConfig, sampled chan *model.Trace) *Sampler {
	return &Sampler{
		engine:  sampler.NewScoreEngine(conf.ExtraSampleRate, conf.MaxTPS),
		sampled: sampled,
	}
}

// NewPrioritySampler creates a new empty distributed sampler ready to be started
func NewPrioritySampler(conf *config.AgentConfig, dynConf *config.DynamicConfig, sampled chan *model.Trace) *Sampler {
	return &Sampler{
		engine:  sampler.NewPriorityEngine(conf.ExtraSampleRate, conf.MaxTPS, &dynConf.RateByService),
		sampled: sampled,
	}
}

// Run starts sampling traces
func (s *Sampler) Run() {
	go func() {
		defer watchdog.LogOnPanic()
		s.engine.Run()
	}()

	go func() {
		defer watchdog.LogOnPanic()
		s.logStats()
	}()
}

// Add samples a trace then keep it until the next flush
func (s *Sampler) Add(t processedTrace) {
	s.totalTraceCount++

	if s.engine.Sample(t.Trace, t.Root, t.Env) {
		s.keptTraceCount++
		s.sampled <- &t.Trace
	}
}

// Stop stops the sampler
func (s *Sampler) Stop() {
	s.engine.Stop()

}

// logStats reports statistics and update the info exposed.
func (s *Sampler) logStats() {

	for now := range time.Tick(10 * time.Second) {
		keptTraceCount := s.keptTraceCount
		totalTraceCount := s.totalTraceCount
		s.keptTraceCount = 0
		s.totalTraceCount = 0

		duration := now.Sub(s.lastFlush)
		s.lastFlush = now

		// TODO: do we still want that? figure out how it conflicts with what the `state` exposes / what is public metrics.
		var stats info.SamplerStats
		if duration > 0 {
			stats.KeptTPS = float64(keptTraceCount) / duration.Seconds()
			stats.TotalTPS = float64(totalTraceCount) / duration.Seconds()
		}
		engineType := fmt.Sprint(reflect.TypeOf(s.engine))
		log.Debugf("%s: flushed %d sampled traces out of %d", engineType, keptTraceCount, totalTraceCount)

		state := s.engine.GetState()

		switch state := state.(type) {
		case sampler.InternalState:
			log.Debugf("%s: inTPS: %f, outTPS: %f, maxTPS: %f, offset: %f, slope: %f, cardinality: %d",
				engineType, state.InTPS, state.OutTPS, state.MaxTPS, state.Offset, state.Slope, state.Cardinality)

			// publish through expvar
			// TODO: avoid type switch, prefer engine method
			switch s.engine.(type) {
			case *sampler.ScoreEngine:
				info.UpdateSamplerInfo(info.SamplerInfo{EngineType: engineType, Stats: stats, State: state})
			case *sampler.PriorityEngine:
				info.UpdatePrioritySamplerInfo(info.SamplerInfo{EngineType: engineType, Stats: stats, State: state})
			}
		default:
			log.Debugf("unhandled sampler engine, can't log state")
		}
	}
}

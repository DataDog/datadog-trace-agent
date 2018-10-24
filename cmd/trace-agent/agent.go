package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/DataDog/datadog-trace-agent/agent"
	"github.com/DataDog/datadog-trace-agent/api"
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/filters"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/obfuscate"
	"github.com/DataDog/datadog-trace-agent/osutil"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	"github.com/DataDog/datadog-trace-agent/writer"
	log "github.com/cihub/seelog"
)

const processStatsInterval = time.Minute

type processedTrace struct {
	Trace         agent.Trace
	WeightedTrace agent.WeightedTrace
	Root          *agent.Span
	Env           string
	Sublayers     map[*agent.Span][]agent.SublayerValue
}

func (pt *processedTrace) weight() float64 {
	if pt.Root == nil {
		return 1.0
	}
	return pt.Root.Weight()
}

func (pt *processedTrace) getSamplingPriority() (int, bool) {
	if pt.Root == nil {
		return 0, false
	}
	p, ok := pt.Root.Metrics[sampler.SamplingPriorityKey]
	return int(p), ok
}

// Agent struct holds all the sub-routines structs and make the data flow between them
type Agent struct {
	Receiver           *api.HTTPReceiver
	Concentrator       *Concentrator
	Blacklister        *filters.Blacklister
	Replacer           *filters.Replacer
	ScoreSampler       *Sampler
	ErrorsScoreSampler *Sampler
	PrioritySampler    *Sampler
	TransactionSampler TransactionSampler
	TraceWriter        *writer.TraceWriter
	ServiceWriter      *writer.ServiceWriter
	StatsWriter        *writer.StatsWriter
	ServiceExtractor   *TraceServiceExtractor
	ServiceMapper      *ServiceMapper

	// obfuscator is used to obfuscate sensitive data from various span
	// tags based on their type.
	obfuscator *obfuscate.Obfuscator

	sampledTraceChan chan *writer.SampledTrace

	// config
	conf    *config.AgentConfig
	dynConf *config.DynamicConfig

	// Used to synchronize on a clean exit
	ctx context.Context
}

// NewAgent returns a new Agent object, ready to be started. It takes a context
// which may be cancelled in order to gracefully stop the agent.
func NewAgent(ctx context.Context, conf *config.AgentConfig) *Agent {
	dynConf := config.NewDynamicConfig()

	// inter-component channels
	rawTraceChan := make(chan agent.Trace, 5000) // about 1000 traces/sec for 5 sec, TODO: move to *agent.Trace
	sampledTraceChan := make(chan *writer.SampledTrace)
	statsChan := make(chan []agent.StatsBucket)
	serviceChan := make(chan agent.ServicesMetadata, 50)
	filteredServiceChan := make(chan agent.ServicesMetadata, 50)

	// create components
	r := api.NewHTTPReceiver(conf, dynConf, rawTraceChan, serviceChan)
	c := NewConcentrator(
		conf.ExtraAggregators,
		conf.BucketInterval.Nanoseconds(),
		statsChan,
	)

	obf := obfuscate.NewObfuscator(conf.Obfuscation)
	ss := NewScoreSampler(conf)
	ess := NewErrorsSampler(conf)
	ps := NewPrioritySampler(conf, dynConf)
	ts := NewTransactionSampler(conf)
	se := NewTraceServiceExtractor(serviceChan)
	sm := NewServiceMapper(serviceChan, filteredServiceChan)
	tw := writer.NewTraceWriter(conf, sampledTraceChan)
	sw := writer.NewStatsWriter(conf, statsChan)
	svcW := writer.NewServiceWriter(conf, filteredServiceChan)

	return &Agent{
		Receiver:           r,
		Concentrator:       c,
		Blacklister:        filters.NewBlacklister(conf.Ignore["resource"]),
		Replacer:           filters.NewReplacer(conf.ReplaceTags),
		ScoreSampler:       ss,
		ErrorsScoreSampler: ess,
		PrioritySampler:    ps,
		TransactionSampler: ts,
		TraceWriter:        tw,
		StatsWriter:        sw,
		ServiceWriter:      svcW,
		ServiceExtractor:   se,
		ServiceMapper:      sm,
		obfuscator:         obf,
		sampledTraceChan:   sampledTraceChan,
		conf:               conf,
		dynConf:            dynConf,
		ctx:                ctx,
	}
}

// Run starts routers routines and individual pieces then stop them when the exit order is received
func (a *Agent) Run() {
	// it's really important to use a ticker for this, and with a not too short
	// interval, for this is our guarantee that the process won't start and kill
	// itself too fast (nightmare loop)
	watchdogTicker := time.NewTicker(a.conf.WatchdogInterval)
	defer watchdogTicker.Stop()

	// update the data served by expvar so that we don't expose a 0 sample rate
	info.UpdatePreSampler(*a.Receiver.PreSampler.Stats())

	// TODO: unify components APIs. Use Start/Stop as non-blocking ways of controlling the blocking Run loop.
	// Like we do with TraceWriter.
	a.Receiver.Run()
	a.TraceWriter.Start()
	a.StatsWriter.Start()
	a.ServiceMapper.Start()
	a.ServiceWriter.Start()
	a.Concentrator.Start()
	a.ScoreSampler.Run()
	a.ErrorsScoreSampler.Run()
	a.PrioritySampler.Run()

	for {
		select {
		case t := <-a.Receiver.Out:
			a.Process(t)
		case <-watchdogTicker.C:
			a.watchdog()
		case <-a.ctx.Done():
			log.Info("exiting")
			if err := a.Receiver.Stop(); err != nil {
				log.Error(err)
			}
			a.Concentrator.Stop()
			a.TraceWriter.Stop()
			a.StatsWriter.Stop()
			a.ServiceMapper.Stop()
			a.ServiceWriter.Stop()
			a.ScoreSampler.Stop()
			a.ErrorsScoreSampler.Stop()
			a.PrioritySampler.Stop()
			return
		}
	}
}

// Process is the default work unit that receives a trace, transforms it and
// passes it downstream.
func (a *Agent) Process(t agent.Trace) {
	if len(t) == 0 {
		log.Debugf("skipping received empty trace")
		return
	}

	// Root span is used to carry some trace-level metadata, such as sampling rate and priority.
	root := t.GetRoot()

	// We get the address of the struct holding the stats associated to no tags.
	// TODO: get the real tagStats related to this trace payload (per lang/version).
	ts := a.Receiver.Stats.GetTagStats(info.Tags{})

	// Extract priority early, as later goroutines might manipulate the Metrics map in parallel which isn't safe.
	priority, hasPriority := root.Metrics[sampler.SamplingPriorityKey]

	// Depending on the sampling priority, count that trace differently.
	stat := &ts.TracesPriorityNone
	if hasPriority {
		if priority < 0 {
			stat = &ts.TracesPriorityNeg
		} else if priority == 0 {
			stat = &ts.TracesPriority0
		} else if priority == 1 {
			stat = &ts.TracesPriority1
		} else {
			stat = &ts.TracesPriority2
		}
	}
	atomic.AddInt64(stat, 1)

	if !a.Blacklister.Allows(root) {
		log.Debugf("trace rejected by blacklister. root: %v", root)
		atomic.AddInt64(&ts.TracesFiltered, 1)
		atomic.AddInt64(&ts.SpansFiltered, int64(len(t)))
		return
	}

	// Extra sanitization steps of the trace.
	for _, span := range t {
		a.obfuscator.Obfuscate(span)
		span.Truncate()
	}
	a.Replacer.Replace(&t)

	// Extract the client sampling rate.
	clientSampleRate := sampler.GetTraceAppliedSampleRate(root)
	// Combine it with the pre-sampling rate.
	preSamplerRate := a.Receiver.PreSampler.Rate()
	// Combine them and attach it to the root to be used for weighing.
	sampler.SetTraceAppliedSampleRate(root, clientSampleRate*preSamplerRate)

	// Figure out the top-level spans and sublayers now as it involves modifying the Metrics map
	// which is not thread-safe while samplers and Concentrator might modify it too.
	t.ComputeTopLevel()

	subtraces := t.ExtractTopLevelSubtraces(root)
	sublayers := make(map[*agent.Span][]agent.SublayerValue)
	for _, subtrace := range subtraces {
		subtraceSublayers := agent.ComputeSublayers(subtrace.Trace)
		sublayers[subtrace.Root] = subtraceSublayers
		agent.SetSublayersOnSpan(subtrace.Root, subtraceSublayers)
	}

	pt := processedTrace{
		Trace:         t,
		WeightedTrace: agent.NewWeightedTrace(t, root),
		Root:          root,
		Env:           a.conf.DefaultEnv,
		Sublayers:     sublayers,
	}
	// Replace Agent-configured environment with `env` coming from span tag.
	if tenv := t.GetEnv(); tenv != "" {
		pt.Env = tenv
	}

	go func() {
		defer watchdog.LogOnPanic()
		a.ServiceExtractor.Process(pt.WeightedTrace)
	}()

	go func() {
		defer watchdog.LogOnPanic()
		// Everything is sent to concentrator for stats, regardless of sampling.
		a.Concentrator.Add(pt)
	}()

	// Don't go through sampling for < 0 priority traces
	if priority < 0 {
		return
	}
	// Run both full trace sampling and transaction extraction in another goroutine.
	go func() {
		defer watchdog.LogOnPanic()

		// All traces should go through either through the normal score sampler or
		// the one dedicated to errors.
		samplers := make([]*Sampler, 0, 2)
		if traceContainsError(t) {
			samplers = append(samplers, a.ErrorsScoreSampler)
		} else {
			samplers = append(samplers, a.ScoreSampler)
		}
		if hasPriority {
			// If Priority is defined, send to priority sampling, regardless of priority value.
			// The sampler will keep or discard the trace, but we send everything so that it
			// gets the big picture and can set the sampling rates accordingly.
			samplers = append(samplers, a.PrioritySampler)
		}

		// Trace sampling.
		var sampledTrace writer.SampledTrace

		sampled := false
		for _, s := range samplers {
			// Consider trace as sampled if at least one of the samplers kept it.
			sampled = s.Add(pt) || sampled
		}
		if sampled {
			sampledTrace.Trace = &pt.Trace
		}

		sampledTrace.Transactions = a.TransactionSampler.Extract(pt)
		// TODO: attach to these transactions the client, pre-sampler and transaction sample rates.

		if !sampledTrace.Empty() {
			a.sampledTraceChan <- &sampledTrace
		}
	}()
}

// dieFunc is used by watchdog to kill the agent; replaced in tests.
var dieFunc = func(fmt string, args ...interface{}) {
	osutil.Exitf(fmt, args...)
}

func (a *Agent) watchdog() {
	var wi watchdog.Info
	wi.CPU = watchdog.CPU()
	wi.Mem = watchdog.Mem()
	wi.Net = watchdog.Net()

	if float64(wi.Mem.Alloc) > a.conf.MaxMemory && a.conf.MaxMemory > 0 {
		dieFunc("exceeded max memory (current=%d, max=%d)", wi.Mem.Alloc, int64(a.conf.MaxMemory))
	}
	if int(wi.Net.Connections) > a.conf.MaxConnections && a.conf.MaxConnections > 0 {
		dieFunc("exceeded max connections (current=%d, max=%d)", wi.Net.Connections, a.conf.MaxConnections)
	}

	info.UpdateWatchdogInfo(wi)

	// Adjust pre-sampling dynamically
	rate, err := sampler.CalcPreSampleRate(a.conf.MaxCPU, wi.CPU.UserAvg, a.Receiver.PreSampler.RealRate())
	if err != nil {
		log.Warnf("problem computing pre-sample rate: %v", err)
	}
	a.Receiver.PreSampler.SetRate(rate)
	a.Receiver.PreSampler.SetError(err)

	preSamplerStats := a.Receiver.PreSampler.Stats()
	statsd.Client.Gauge("datadog.trace_agent.presampler_rate", preSamplerStats.Rate, nil, 1)
	info.UpdatePreSampler(*preSamplerStats)
}

func traceContainsError(trace agent.Trace) bool {
	for _, span := range trace {
		if span.Error != 0 {
			return true
		}
	}
	return false
}

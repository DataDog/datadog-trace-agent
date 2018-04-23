package main

import (
	"context"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/filters"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/quantizer"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	"github.com/DataDog/datadog-trace-agent/writer"
)

const (
	processStatsInterval = time.Minute
	samplingPriorityKey  = "_sampling_priority_v1"
)

type processedTrace struct {
	Trace         model.Trace
	WeightedTrace model.WeightedTrace
	Root          *model.Span
	Env           string
	Sublayers     map[*model.Span][]model.SublayerValue
}

func (pt *processedTrace) weight() float64 {
	if pt.Root == nil {
		return 1.0
	}
	return pt.Root.Weight()
}

// Agent struct holds all the sub-routines structs and make the data flow between them
type Agent struct {
	Receiver           *HTTPReceiver
	Concentrator       *Concentrator
	Filters            []filters.Filter
	ScoreSampler       *Sampler
	PrioritySampler    *Sampler
	TransactionSampler *TransactionSampler
	TraceWriter        *writer.TraceWriter
	ServiceWriter      *writer.ServiceWriter
	StatsWriter        *writer.StatsWriter
	ServiceExtractor   *TraceServiceExtractor
	ServiceMapper      *ServiceMapper

	sampledTraceChan chan *model.Trace

	// config
	conf    *config.AgentConfig
	dynConf *config.DynamicConfig

	// Used to synchronize on a clean exit
	ctx context.Context

	die func(format string, args ...interface{})
}

// NewAgent returns a new Agent object, ready to be started. It takes a context
// which may be cancelled in order to gracefully stop the agent.
func NewAgent(ctx context.Context, conf *config.AgentConfig) *Agent {
	dynConf := config.NewDynamicConfig()

	// inter-component channels
	rawTraceChan := make(chan model.Trace, 5000) // about 1000 traces/sec for 5 sec, TODO: move to *model.Trace
	sampledTraceChan := make(chan *model.Trace)
	analyzedTransactionChan := make(chan *model.Span)
	statsChan := make(chan []model.StatsBucket)
	serviceChan := make(chan model.ServicesMetadata, 50)
	filteredServiceChan := make(chan model.ServicesMetadata, 50)

	// create components
	r := NewHTTPReceiver(conf, dynConf, rawTraceChan, serviceChan)
	c := NewConcentrator(
		conf.ExtraAggregators,
		conf.BucketInterval.Nanoseconds(),
		statsChan,
	)
	f := filters.Setup(conf)

	ss := NewScoreSampler(conf)
	ps := NewPrioritySampler(conf, dynConf)
	ts := NewTransactionSampler(conf, analyzedTransactionChan)
	se := NewTraceServiceExtractor(serviceChan)
	sm := NewServiceMapper(serviceChan, filteredServiceChan)
	tw := writer.NewTraceWriter(conf, sampledTraceChan, analyzedTransactionChan)
	sw := writer.NewStatsWriter(conf, statsChan)
	svcW := writer.NewServiceWriter(conf, filteredServiceChan)

	return &Agent{
		Receiver:           r,
		Concentrator:       c,
		Filters:            f,
		ScoreSampler:       ss,
		PrioritySampler:    ps,
		TransactionSampler: ts,
		TraceWriter:        tw,
		StatsWriter:        sw,
		ServiceWriter:      svcW,
		ServiceExtractor:   se,
		ServiceMapper:      sm,
		sampledTraceChan:   sampledTraceChan,
		conf:               conf,
		dynConf:            dynConf,
		ctx:                ctx,
		die:                die,
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
	info.UpdatePreSampler(*a.Receiver.preSampler.Stats())

	// TODO: unify components APIs. Use Start/Stop as non-blocking ways of controlling the blocking Run loop.
	// Like we do with TraceWriter.
	a.Receiver.Run()
	a.TraceWriter.Start()
	a.StatsWriter.Start()
	a.ServiceMapper.Start()
	a.ServiceWriter.Start()
	a.Concentrator.Start()
	a.ScoreSampler.Run()
	a.PrioritySampler.Run()

	for {
		select {
		case t := <-a.Receiver.traces:
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
			a.PrioritySampler.Stop()
			return
		}
	}
}

// Process is the default work unit that receives a trace, transforms it and
// passes it downstream.
func (a *Agent) Process(t model.Trace) {
	if len(t) == 0 {
		// XXX Should never happen since we reject empty traces during
		// normalization.
		log.Debugf("skipping received empty trace")
		return
	}

	root := t.GetRoot()

	// We get the address of the struct holding the stats associated to no tags
	// TODO: get the real tagStats related to this trace payload.
	ts := a.Receiver.stats.GetTagStats(info.Tags{})

	samplers := []*Sampler{
		// Always use score sampler so it has a real idea of trace distribution
		a.ScoreSampler,
	}

	priority, hasPriority := root.Metrics[samplingPriorityKey]
	if hasPriority {
		// If Priority is defined, send to priority sampling, regardless of priority value.
		// The sampler will keep or discard the trace, but we send everything so that it
		// gets the big picture and can set the sampling rates accordingly.
		samplers = append(samplers, a.PrioritySampler)
	}

	priorityPtr := &ts.TracesPriorityNone
	if hasPriority {
		if priority < 0 {
			priorityPtr = &ts.TracesPriorityNeg
		} else if priority == 0 {
			priorityPtr = &ts.TracesPriority0
		} else if priority == 1 {
			priorityPtr = &ts.TracesPriority1
		} else {
			priorityPtr = &ts.TracesPriority2
		}
	}
	atomic.AddInt64(priorityPtr, 1)

	if root.End() < model.Now()-2*a.conf.BucketInterval.Nanoseconds() {
		log.Errorf("skipping trace with root too far in past, root:%v", *root)

		atomic.AddInt64(&ts.TracesDropped, 1)
		atomic.AddInt64(&ts.SpansDropped, int64(len(t)))
		return
	}

	for _, f := range a.Filters {
		if f.Keep(root, &t) {
			continue
		}

		log.Debugf("rejecting trace by filter: %T  %v", f, *root)
		atomic.AddInt64(&ts.TracesFiltered, 1)
		atomic.AddInt64(&ts.SpansFiltered, int64(len(t)))

		return
	}

	rate := sampler.GetTraceAppliedSampleRate(root)
	rate *= a.Receiver.preSampler.Rate()
	sampler.SetTraceAppliedSampleRate(root, rate)

	// Need to do this computation before entering the concentrator
	// as they access the Metrics map, which is not thread safe.
	t.ComputeTopLevel()
	wt := model.NewWeightedTrace(t, root)

	subtraces := t.ExtractTopLevelSubtraces(root)
	sublayers := make(map[*model.Span][]model.SublayerValue)
	for _, subtrace := range subtraces {
		subtraceSublayers := model.ComputeSublayers(subtrace.Trace)
		sublayers[subtrace.Root] = subtraceSublayers
		model.SetSublayersOnSpan(subtrace.Root, subtraceSublayers)
	}

	for i := range t {
		quantizer.Quantize(t[i])
		t[i].Truncate()
	}

	pt := processedTrace{
		Trace:         t,
		WeightedTrace: wt,
		Root:          root,
		Env:           a.conf.DefaultEnv,
		Sublayers:     sublayers,
	}
	if tenv := t.GetEnv(); tenv != "" {
		pt.Env = tenv
	}

	go func() {
		defer watchdog.LogOnPanic()
		a.ServiceExtractor.Process(wt)
	}()

	go func() {
		defer watchdog.LogOnPanic()
		// Everything is sent to concentrator for stats, regardless of sampling.
		a.Concentrator.Add(pt)

	}()
	if hasPriority && priority < 0 {
		// If the trace has a negative priority we absolutely don't want it
		// sampled either by the trace or transaction pipeline so we return here
		return
	}
	go func() {
		defer watchdog.LogOnPanic()
		sampled := false

		for _, s := range samplers {
			// Consider trace as sampled if at least one of the samplers kept it
			sampled = s.Add(pt) || sampled
		}

		if sampled {
			a.sampledTraceChan <- &pt.Trace
		}
	}()
	if a.TransactionSampler.Enabled() {
		go func() {
			defer watchdog.LogOnPanic()
			a.TransactionSampler.Add(pt)
		}()
	}
}

func (a *Agent) watchdog() {
	var wi watchdog.Info
	wi.CPU = watchdog.CPU()
	wi.Mem = watchdog.Mem()
	wi.Net = watchdog.Net()

	if float64(wi.Mem.Alloc) > a.conf.MaxMemory && a.conf.MaxMemory > 0 {
		a.die("exceeded max memory (current=%d, max=%d)", wi.Mem.Alloc, int64(a.conf.MaxMemory))
	}
	if int(wi.Net.Connections) > a.conf.MaxConnections && a.conf.MaxConnections > 0 {
		a.die("exceeded max connections (current=%d, max=%d)", wi.Net.Connections, a.conf.MaxConnections)
	}

	info.UpdateWatchdogInfo(wi)

	// Adjust pre-sampling dynamically
	rate, err := sampler.CalcPreSampleRate(a.conf.MaxCPU, wi.CPU.UserAvg, a.Receiver.preSampler.RealRate())
	if rate > a.conf.PreSampleRate {
		rate = a.conf.PreSampleRate
	}
	if err != nil {
		log.Warnf("problem computing pre-sample rate: %v", err)
	}
	a.Receiver.preSampler.SetRate(rate)
	a.Receiver.preSampler.SetError(err)

	info.UpdatePreSampler(*a.Receiver.preSampler.Stats())
}

package main

import (
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
	Sublayers     []model.SublayerValue
}

func (pt *processedTrace) weight() float64 {
	if pt.Root == nil {
		return 1.0
	}
	return pt.Root.Weight()
}

// Agent struct holds all the sub-routines structs and make the data flow between them
type Agent struct {
	Receiver        *HTTPReceiver
	Concentrator    *Concentrator
	Filters         []filters.Filter
	ScoreSampler    *Sampler
	PrioritySampler *Sampler
	TraceWriter     *writer.TraceWriter
	ServiceWriter   *writer.ServiceWriter
	StatsWriter     *writer.StatsWriter

	// config
	conf    *config.AgentConfig
	dynConf *config.DynamicConfig

	// Used to synchronize on a clean exit
	exit chan struct{}

	die func(format string, args ...interface{})
}

// NewAgent returns a new Agent object, ready to be started
func NewAgent(conf *config.AgentConfig, exit chan struct{}) *Agent {
	dynConf := config.NewDynamicConfig()

	// inter-component channels
	rawTraceChan := make(chan model.Trace, 5000) // about 1000 traces/sec for 5 sec, TODO: move to *model.Trace
	sampledTraceChan := make(chan *model.Trace)
	analyzedTransactionChan := make(chan *model.Span)
	statsChan := make(chan []model.StatsBucket)
	serviceChan := make(chan model.ServicesMetadata, 50)

	// create components
	r := NewHTTPReceiver(conf, dynConf, rawTraceChan, serviceChan)
	c := NewConcentrator(
		conf.ExtraAggregators,
		conf.BucketInterval.Nanoseconds(),
		statsChan,
	)
	f := filters.Setup(conf)

	ss := NewScoreSampler(conf, sampledTraceChan, analyzedTransactionChan)
	ps := NewPrioritySampler(conf, dynConf, sampledTraceChan, analyzedTransactionChan)
	tw := writer.NewTraceWriter(conf, sampledTraceChan, analyzedTransactionChan)
	sw := writer.NewStatsWriter(conf, statsChan)
	svcW := writer.NewServiceWriter(conf, serviceChan)

	// wire components together
	tw.InTraces = sampledTraceChan
	sw.InStats = statsChan
	svcW.InServices = serviceChan

	return &Agent{
		Receiver:        r,
		Concentrator:    c,
		Filters:         f,
		ScoreSampler:    ss,
		PrioritySampler: ps,
		TraceWriter:     tw,
		StatsWriter:     sw,
		ServiceWriter:   svcW,
		conf:            conf,
		dynConf:         dynConf,
		exit:            exit,
		die:             die,
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
		case <-a.exit:
			log.Info("exiting")
			close(a.Receiver.exit)
			a.Concentrator.Stop()
			a.TraceWriter.Stop()
			a.StatsWriter.Stop()
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

	var samplers []*Sampler
	priority, ok := root.Metrics[samplingPriorityKey]
	// Send traces to possibly several score engines.
	if ok {
		// If Priority is defined, send to priority sampling, regardless of priority value.
		// The sampler will keep or discard the trace, but we send everything so that it
		// gets the big picture and can set the sampling rates accordingly.
		samplers = append(samplers, a.PrioritySampler)
	}
	if priority == 0 {
		// Use score engine for traces with no priority or priority set to 0
		samplers = append(samplers, a.ScoreSampler)
	}

	priorityPtr := &ts.TracesPriorityNone
	if ok {
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
		if f.Keep(root) {
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

	sublayers := model.ComputeSublayers(t)
	model.SetSublayersOnSpan(root, sublayers)

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
		// Everything is sent to concentrator for stats, regardless of sampling.
		a.Concentrator.Add(pt)

	}()
	for _, s := range samplers {
		sampler := s
		go func() {
			defer watchdog.LogOnPanic()
			sampler.Add(pt)
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

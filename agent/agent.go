package main

import (
	"sync"
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
	languageHeaderKey    = "X-Datadog-Reported-Languages"
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
	Receiver       *HTTPReceiver
	Concentrator   *Concentrator
	Filters        []filters.Filter
	ScoreEngine    *Sampler
	PriorityEngine *Sampler
	Writer         *writer.Writer

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
	r := NewHTTPReceiver(conf, dynConf)
	c := NewConcentrator(
		conf.ExtraAggregators,
		conf.BucketInterval.Nanoseconds(),
	)
	f := filters.Setup(conf)
	ss := NewScoreEngine(conf)
	var ps *Sampler
	if conf.PrioritySampling {
		// Use priority sampling for distributed tracing only if conf says so
		ps = NewPriorityEngine(conf, dynConf)
	}

	w := writer.NewWriter(conf)
	w.InServices = r.services

	return &Agent{
		Receiver:       r,
		Concentrator:   c,
		Filters:        f,
		ScoreEngine:    ss,
		PriorityEngine: ps,
		Writer:         w,
		conf:           conf,
		dynConf:        dynConf,
		exit:           exit,
		die:            die,
	}
}

// Run starts routers routines and individual pieces then stop them when the exit order is received
func (a *Agent) Run() {
	flushTicker := time.NewTicker(a.conf.BucketInterval)
	defer flushTicker.Stop()

	// it's really important to use a ticker for this, and with a not too short
	// interval, for this is our garantee that the process won't start and kill
	// itself too fast (nightmare loop)
	watchdogTicker := time.NewTicker(a.conf.WatchdogInterval)
	defer watchdogTicker.Stop()

	// update the data served by expvar so that we don't expose a 0 sample rate
	info.UpdatePreSampler(*a.Receiver.preSampler.Stats())

	a.Receiver.Run()
	a.Writer.Run()
	a.ScoreEngine.Run()
	if a.PriorityEngine != nil {
		a.PriorityEngine.Run()
	}

	for {
		select {
		case t := <-a.Receiver.traces:
			a.Process(t)
		case <-flushTicker.C:
			p := model.AgentPayload{
				HostName: a.conf.HostName,
				Env:      a.conf.DefaultEnv,
			}
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer watchdog.LogOnPanic()
				p.Stats = a.Concentrator.Flush()
				wg.Done()
			}()
			go func() {
				defer watchdog.LogOnPanic()
				// Serializing both flushes, classic agent sampler and distributed sampler,
				// in most cases only one will be used, so in mainstream case there should
				// be no performance issue, only in transitionnal mode can both contain data.
				p.Traces = a.ScoreEngine.Flush()
				if a.PriorityEngine != nil {
					p.Traces = append(p.Traces, a.PriorityEngine.Flush()...)
				}
				wg.Done()
			}()

			wg.Wait()
			p.SetExtra(languageHeaderKey, a.Receiver.Languages())

			a.Writer.InPayloads <- p
		case <-watchdogTicker.C:
			a.watchdog()
		case <-a.exit:
			log.Info("exiting")
			close(a.Receiver.exit)
			a.Writer.Stop()
			a.ScoreEngine.Stop()
			if a.PriorityEngine != nil {
				a.PriorityEngine.Stop()
			}
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

	// We choose the sampler dynamically, depending on trace content,
	// it has a sampling priority info (wether 0 or 1 or more) we respect
	// this by using priority sampler. Else, use default score sampler.
	s := a.ScoreEngine
	priorityPtr := &ts.TracesPriorityNone
	if a.PriorityEngine != nil {
		if priority, ok := root.Metrics[samplingPriorityKey]; ok {
			s = a.PriorityEngine

			if priority == 0 {
				priorityPtr = &ts.TracesPriority0
			} else if priority == 1 {
				priorityPtr = &ts.TracesPriority1
			} else {
				priorityPtr = &ts.TracesPriority2
			}
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
		a.Concentrator.Add(pt)

	}()
	go func() {
		defer watchdog.LogOnPanic()
		s.Add(pt)
	}()
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

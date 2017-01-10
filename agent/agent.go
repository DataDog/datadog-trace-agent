package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/quantizer"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	log "github.com/cihub/seelog"
)

const processStatsInterval = time.Minute

type processedTrace struct {
	Trace     model.Trace
	Root      *model.Span
	Env       string
	Sublayers []model.SublayerValue
}

func (pt *processedTrace) weight() float64 {
	if pt.Root == nil {
		return 1.0
	}
	return pt.Root.Weight()
}

// Agent struct holds all the sub-routines structs and make the data flow between them
type Agent struct {
	Receiver     *HTTPReceiver
	Concentrator *Concentrator
	Sampler      *Sampler
	Writer       *Writer

	// config
	conf *config.AgentConfig

	// Used to synchronize on a clean exit
	exit chan struct{}
}

// NewAgent returns a new Agent object, ready to be started
func NewAgent(conf *config.AgentConfig) *Agent {
	exit := make(chan struct{})

	r := NewHTTPReceiver(conf)
	c := NewConcentrator(
		conf.ExtraAggregators,
		conf.BucketInterval.Nanoseconds(),
	)
	s := NewSampler(conf)

	w := NewWriter(conf)
	w.inServices = r.services

	return &Agent{
		Receiver:     r,
		Concentrator: c,
		Sampler:      s,
		Writer:       w,
		conf:         conf,
		exit:         exit,
	}
}

// Run starts routers routines and individual pieces then stop them when the exit order is received
func (a *Agent) Run() {
	flushTicker := time.NewTicker(a.conf.BucketInterval)
	defer flushTicker.Stop()
	watchdogTicker := time.NewTicker(a.conf.WatchdogInterval)
	defer watchdogTicker.Stop()

	a.Receiver.Run()
	a.Writer.Run()
	a.Sampler.Run()

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
				p.Stats = a.Concentrator.Flush()
				wg.Done()
			}()
			go func() {
				p.Traces = a.Sampler.Flush()
				wg.Done()
			}()

			wg.Wait()

			a.Writer.inPayloads <- p
		case <-watchdogTicker.C:
			a.watchdog()
		case <-a.exit:
			log.Info("exiting")
			close(a.Receiver.exit)
			a.Writer.Stop()
			a.Sampler.Stop()
			return
		}
	}
}

// Process is the default work unit that receives a trace, transforms it and
// passes it downstream
func (a *Agent) Process(t model.Trace) {
	if len(t) == 0 {
		// XXX Should never happen since we reject empty traces during
		// normalization.
		log.Debugf("skipping received empty trace")
		return
	}

	root := t.GetRoot()
	if root.End() < model.Now()-2*a.conf.BucketInterval.Nanoseconds() {
		log.Debugf("skipping trace with root too far in past, root:%v", *root)
		atomic.AddInt64(&a.Receiver.stats.TracesDropped, 1)
		atomic.AddInt64(&a.Receiver.stats.SpansDropped, int64(len(t)))
		return
	}

	sublayers := model.ComputeSublayers(&t)
	model.SetSublayersOnSpan(root, sublayers)

	for i := range t {
		t[i] = quantizer.Quantize(t[i])
	}

	pt := processedTrace{
		Trace:     t,
		Root:      root,
		Env:       a.conf.DefaultEnv,
		Sublayers: sublayers,
	}
	if tenv := t.GetEnv(); tenv != "" {
		pt.Env = tenv
	}

	weight := pt.weight() // need to do this now because sampler edits .Metrics map
	go a.Concentrator.Add(pt, weight)
	go a.Sampler.Add(pt)
}

func (a *Agent) watchdog() {
	tags := []string{"version:" + Version, "go_version:" + GoVersion}
	c := watchdog.CPU()
	statsd.Client.Gauge("trace_agent.process.cpu.pct", c.UserAvg*100.0, tags, 1)
	m := watchdog.Mem()
	statsd.Client.Gauge("trace_agent.process.mem.alloc", float64(m.Alloc), tags, 1)
	statsd.Client.Gauge("trace_agent.process.mem.alloc_per_sec", m.AllocPerSec, tags, 1)

	if float64(m.Alloc) > a.conf.MaxMemory {
		msg := fmt.Sprintf("exceeded max memory (current=%d, max=%d)", m.Alloc, int64(a.conf.MaxMemory))
		log.Error(msg)
		panic(msg)
	}
}

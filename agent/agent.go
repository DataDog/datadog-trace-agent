package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/quantizer"
	log "github.com/cihub/seelog"
)

type processedTrace struct {
	Trace     model.Trace
	Root      *model.Span
	Env       string
	Sublayers []model.SublayerValue
}

// AgentStatus is used to store information about the agent and is used for
// the response body in the /status handler.
type AgentStatus struct {
	Version   string `json:"version"`
	BuildDate string `json:"build_date"`
	GitCommit string `json:"git_commit"`
	GitBranch string `json:"git_branch"`
	GoVersion string `json:"go_version"`

	Running bool `json:"running"`

	*sync.Mutex
}

// Agent struct holds all the sub-routines structs and make the data flow between them
type Agent struct {
	Receiver     *HTTPReceiver
	Concentrator *Concentrator
	Sampler      *Sampler
	Writer       *Writer

	status AgentStatus

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

	status := AgentStatus{
		Version:   Version,
		BuildDate: BuildDate,
		GitCommit: GitCommit,
		GitBranch: GitBranch,
		GoVersion: GoVersion,

		Mutex: &sync.Mutex{},
	}

	agent := &Agent{
		Receiver:     r,
		Concentrator: c,
		Sampler:      s,
		Writer:       w,

		status: status,

		conf: conf,
		exit: exit,
	}

	r.agent = agent

	return agent
}

// Run starts routers routines and individual pieces then stop them when the exit order is received
func (a *Agent) Run() {
	flushTicker := time.NewTicker(a.conf.BucketInterval)
	defer flushTicker.Stop()

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

	// NOTE: right now we don't use the .Metrics map in the concentrator
	// but if we did, it would be racy with the Sampler that edits it
	go a.Concentrator.Add(pt)
	go a.Sampler.Add(pt)
}

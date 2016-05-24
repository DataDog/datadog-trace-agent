package main

import (
	"sync"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	log "github.com/cihub/seelog"
)

// Agent struct holds all the sub-routines structs and make the data flow between them
type Agent struct {
	Receiver     Receiver // Receiver is an interface
	Quantizer    *Quantizer
	Concentrator *Concentrator
	Sampler      *Sampler
	Writer       *Writer

	// config
	Config         *config.AgentConfig
	traceConsumers int

	// Used to synchronize on a clean exit
	exit chan struct{}
}

// NewAgent returns a new Agent object, ready to be started
func NewAgent(conf *config.AgentConfig) *Agent {
	exit := make(chan struct{})

	r := NewHTTPReceiver()
	q := NewQuantizer(r.traces)

	traceConsumers := 2
	traceChans := traceFanOut(q.out, traceConsumers)

	c := NewConcentrator(traceChans[0], conf)
	s := NewSampler(traceChans[1], conf)

	w := NewWriter(conf, r.services)

	return &Agent{
		Config:         conf,
		Receiver:       r,
		Quantizer:      q,
		Concentrator:   c,
		Sampler:        s,
		Writer:         w,
		traceConsumers: traceConsumers,
		exit:           exit,
	}
}

// Run starts routers routines and individual pieces then stop them when the exit order is received
func (a *Agent) Run() {
	// Start all workers
	go a.runFlusher()
	a.Start()
	// Wait for the exit order
	<-a.exit
	// Stop all workers
	a.Stop()
}

// runFlusher periodically send a flush marker, collect the results and send the payload to the Writer
func (a *Agent) runFlusher() {
	ticker := time.NewTicker(a.Config.BucketInterval)
	for {
		select {
		case <-ticker.C:
			log.Debug("Trigger a flush")
			a.Quantizer.out <- model.NewTraceFlushMarker()

			// Collect and merge partial flushs
			var wg sync.WaitGroup
			p := model.AgentPayload{}
			wg.Add(a.traceConsumers)
			go func() {
				defer wg.Done()
				p.Stats = <-a.Concentrator.out
			}()
			go func() {
				defer wg.Done()
				p.Traces = <-a.Sampler.out
			}()
			wg.Wait()

			if !p.IsEmpty() {
				a.Writer.in <- p
			} else {
				log.Debug("Empty payload, skipping")
			}
		case <-a.exit:
			ticker.Stop()
			return
		}
	}
}

// Start starts all components
func (a *Agent) Start() error {
	log.Info("Starting agent")

	// Build the pipeline in the opposite way the data is processed
	a.Writer.Start()
	a.Sampler.Start()
	a.Concentrator.Start()
	a.Quantizer.Start()
	a.Receiver.Start()

	// FIXME: catch start errors
	return nil
}

// Stop stops all components
func (a *Agent) Stop() error {
	log.Info("Stopping agent")

	a.Receiver.Stop()
	a.Quantizer.Stop()
	a.Concentrator.Stop()
	a.Sampler.Stop()
	a.Writer.Stop()

	return nil
}

// traceFanOut redistributes incoming traces to multiple components by returning multiple channels
func traceFanOut(in chan model.Trace, n int) []chan model.Trace {
	outChans := make([]chan model.Trace, 0, n)
	for i := 0; i < n; i++ {
		outChans = append(outChans, make(chan model.Trace))
	}
	go func() {
		for t := range in {
			for _, outc := range outChans {
				outc <- t
			}
		}
	}()

	return outChans
}

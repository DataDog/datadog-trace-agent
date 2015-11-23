package main

import (
	"sync"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	log "github.com/cihub/seelog"
)

// Agent struct holds all the sub-routines structs and some channels to stream data in those
type Agent struct {
	Receiver     Receiver // Receiver is an interface
	Quantizer    *Quantizer
	Concentrator *Concentrator
	Grapher      *Grapher
	Sampler      *Sampler
	Writer       *Writer

	// config
	Config *config.AgentConfig

	// Used to synchronize on a clean exit
	exit chan struct{}
}

// NewAgent returns a new Agent object, ready to be initialized and started
func NewAgent(conf *config.AgentConfig) *Agent {
	exit := make(chan struct{})

	r := NewHTTPReceiver()
	q := NewQuantizer(r.out)

	spansToConcentrator, spansToGrapher, spansToSampler := spanDoubleTPipe(q.out)

	c := NewConcentrator(spansToConcentrator, conf)
	g := NewGrapher(spansToGrapher, conf)
	s := NewSampler(spansToSampler, conf)

	w := NewWriter(conf)

	return &Agent{
		Config:       conf,
		Receiver:     r,
		Quantizer:    q,
		Concentrator: c,
		Grapher:      g,
		Sampler:      s,
		Writer:       w,
		exit:         exit,
	}
}

// Run starts routers routines and individual pieces forever
func (a *Agent) Run() {
	// Start all workers
	go a.runFlusher()
	a.Start()
	// Wait for the exit order
	<-a.exit
	// Stop all workers
	a.Stop()
}

func (a *Agent) runFlusher() {
	ticker := time.NewTicker(a.Config.BucketInterval)
	for {
		select {
		case <-ticker.C:
			log.Debug("Trigger a flush")
			a.Quantizer.out <- model.NewFlushMarker()

			// Collect and merge partial flushs
			var wg sync.WaitGroup
			p := model.AgentPayload{}
			wg.Add(3)
			go func() {
				defer wg.Done()
				p.Stats = <-a.Concentrator.out
			}()
			go func() {
				defer wg.Done()
				p.Graph = <-a.Grapher.out
			}()
			go func() {
				defer wg.Done()
				p.Spans = <-a.Sampler.out
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

// Start starts routers routines and individual pieces forever
func (a *Agent) Start() error {
	log.Info("Starting agent")

	// Build the pipeline in the opposite way the data is processed
	a.Writer.Start()
	a.Sampler.Start()
	a.Concentrator.Start()
	a.Grapher.Start()
	a.Quantizer.Start()
	a.Receiver.Start()

	// FIXME: catch start errors
	return nil
}

// Stop stops routers routines and individual pieces
func (a *Agent) Stop() error {
	log.Info("Stopping agent")

	a.Receiver.Stop()
	a.Quantizer.Stop()
	a.Concentrator.Stop()
	a.Grapher.Stop()
	a.Sampler.Stop()
	a.Writer.Stop()

	return nil
}

// Distribute spans from the quantizer to the concentrator, grapher and sampler
func spanDoubleTPipe(in chan model.Span) (chan model.Span, chan model.Span, chan model.Span) {
	out1 := make(chan model.Span)
	out2 := make(chan model.Span)
	out3 := make(chan model.Span)

	go func() {
		for s := range in {
			out1 <- s
			out2 <- s
			out3 <- s
		}
	}()

	return out1, out2, out3
}

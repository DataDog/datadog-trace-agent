package main

import (
	"sync"

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
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// NewAgent returns a new Agent object, ready to be initialized and started
func NewAgent(conf *config.AgentConfig) *Agent {

	exit := make(chan struct{})
	var exitGroup sync.WaitGroup

	r := NewHTTPReceiver(exit, &exitGroup)
	q := NewQuantizer(r.out, exit, &exitGroup)

	spansToConcentrator, spansToGrapher, spansToSampler := spanDoubleTPipe(q.out)

	c := NewConcentrator(spansToConcentrator, conf, exit, &exitGroup)
	g := NewGrapher(spansToGrapher, c.out, conf, exit, &exitGroup)
	s := NewSampler(spansToSampler, g.out, conf, exit, &exitGroup)
	w := NewWriter(s.out, conf, exit, &exitGroup)

	return &Agent{
		Config:       conf,
		Receiver:     r,
		Quantizer:    q,
		Concentrator: c,
		Grapher:      g,
		Sampler:      s,
		Writer:       w,
		exit:         exit,
		exitGroup:    &exitGroup,
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

// Join an agent should be called when its exit channel has been closed and waits for sub-routines to return before returning
func (a *Agent) Join() {
	// FIXME: check if the exit channel is closed, otherwise panic as it will never return. Optionally use a timeout here?
	log.Info("Agent stopping, waiting for all running routines to finish")
	a.exitGroup.Wait()
	log.Info("DONE. Exiting now, over and out.")
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

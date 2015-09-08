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
	Writer       *Writer

	// config
	Config *config.File

	// Used to synchronize on a clean exit
	exit      chan struct{}
	exitGroup *sync.WaitGroup

	// internal channels
	spansFromReceiver     chan model.Span
	spansFromQuantizer    chan model.Span
	spansFromConcentrator chan model.Span
	statsFromConcentrator chan model.StatsBucket
}

// NewAgent returns a new Agent object, ready to be initialized and started
func NewAgent(conf *config.File) *Agent {
	endpoint := conf.GetDefault("trace.api", "endpoint", "http://localhost:8012/api/v0.1")

	exit := make(chan struct{})
	var exitGroup sync.WaitGroup

	r, rawSpans := NewHTTPReceiver(exit, &exitGroup)
	q, quantizedSpans := NewQuantizer(rawSpans, exit, &exitGroup)
	c, concentratedSpans, concentratedStats := NewConcentrator(time.Second*5, quantizedSpans, exit, &exitGroup)

	quantiles := []float64{0, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 1}
	w := NewWriter(endpoint, quantiles, concentratedSpans, concentratedStats, exit, &exitGroup)

	return &Agent{
		Config:       conf,
		Receiver:     r,
		Quantizer:    q,
		Concentrator: c,
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

	// sends stuff to the stats writer
	a.Concentrator.Start()

	// sends stuff to our main spansFromQuantizer pipe
	a.Quantizer.Start()

	// sends stuff to our main spansFromReceiver pipe
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

	// Display any messages left in buffers
	log.Flush()
}

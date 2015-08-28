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
	Writer       *Writer

	// config
	Config *config.File

	// Used to synchronize on a clean exit
	exit      chan bool
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

	exit := make(chan bool)
	var exitGroup sync.WaitGroup

	r := NewHTTPReceiver(exit, &exitGroup)
	q := NewQuantizer()
	c := NewConcentrator(
		conf.GetIntDefault("trace.concentrator", "bucket_size", 5),
		conf.GetFloat64Default("trace.concentrator", "quantile_precision", 0),
		exit, &exitGroup)
	w := NewWriter(endpoint, exit, &exitGroup,
		conf.GetIntDefault("trace.sampler", "min_span_by_distribution", 5))

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

// Init needs to be called to initialize channels
func (a *Agent) Init() error {
	log.Info("Initializing agent")
	// Listener initialization
	a.spansFromReceiver = make(chan model.Span)
	a.Receiver.Init(a.spansFromReceiver)

	// Quantizer initialization
	a.spansFromQuantizer = make(chan model.Span)
	a.Quantizer.Init(a.spansFromReceiver, a.spansFromQuantizer, a.exit, a.exitGroup)

	// Concentrator initialization
	a.spansFromConcentrator = make(chan model.Span)
	a.statsFromConcentrator = make(chan model.StatsBucket)
	a.Concentrator.Init(a.spansFromQuantizer, a.statsFromConcentrator, a.spansFromConcentrator)

	// Writer initialization
	a.Writer.Init(a.spansFromConcentrator, a.statsFromConcentrator)

	// FIXME: catch initialization errors
	return nil
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

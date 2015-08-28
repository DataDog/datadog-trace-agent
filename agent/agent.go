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
	Concentrator *Concentrator
	Writer       *Writer

	// config
	Config *config.File

	// Used to synchronize on a clean exit
	exit      chan bool
	exitGroup *sync.WaitGroup

	// internal channels
	spansFromReceiver     chan model.Span
	spansToConcentrator   chan model.Span
	spansFromConcentrator chan model.Span
	statsFromConcentrator chan model.StatsBucket
}

// NewAgent returns a new Agent object, ready to be initialized and started
func NewAgent(conf *config.File) *Agent {
	endpoint := conf.GetDefault("trace.api", "endpoint", "http://localhost:8012/api/v0.1")

	exit := make(chan bool)
	var exitGroup sync.WaitGroup

	r := NewHTTPReceiver(exit, &exitGroup)
	c := NewConcentrator(
		time.Second*5, // FIXME replace with duration parse `5s`
		conf.GetFloat64Default("trace.concentrator", "quantile_precision", 0),
		exit, &exitGroup)

	quantiles := []float64{0, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 1}
	w := NewWriter(endpoint, exit, &exitGroup, quantiles)

	return &Agent{
		Config:       conf,
		Receiver:     r,
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

	// Concentrator initialization
	a.spansToConcentrator = make(chan model.Span)
	a.spansFromConcentrator = make(chan model.Span)
	a.statsFromConcentrator = make(chan model.StatsBucket)
	a.Concentrator.Init(a.spansToConcentrator, a.statsFromConcentrator, a.spansFromConcentrator)

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

	// send stuff to the concentrator and span writer
	// FIXME: might not be needed, keeping it for now in case we need to pipe the traces from the receiver elsewhere
	go func() {
		// should be closed by the listener when exiting
		for s := range a.spansFromReceiver {
			a.spansToConcentrator <- s
		}
		// when no more spans are to be read from the receiver, notify downstream readers
		close(a.spansToConcentrator)
	}()

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

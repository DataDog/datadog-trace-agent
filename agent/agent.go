package main

import (
	"sync"

	"github.com/DataDog/raclette/model"
	log "github.com/cihub/seelog"
)

// Agent struct holds all the sub-routines structs and some channels to stream data in those
type Agent struct {
	Receiver     Receiver // Receiver is an interface
	Concentrator *Concentrator
	SpanWriter   *SpanWriter
	StatsWriter  *StatsWriter

	// Used to synchronize on a clean exit
	exit      chan bool
	exitGroup *sync.WaitGroup

	// internal channels
	spansFromReceiver     chan model.Span
	spansToConcentrator   chan model.Span
	spansToWriter         chan model.Span
	statsFromConcentrator chan model.StatsBucket
}

// NewAgent returns a new Agent object, ready to be initialized and started
func NewAgent() *Agent {
	// FIXME: this should be read from config and not hardcoded
	var concentratorBucketSize int32 = 5
	concentratorStrategy := model.EXACT
	concentratorGKEpsilon := 1e-3
	APIEndpoint := "http://localhost:8012/api/v0.1"

	exit := make(chan bool)
	var exitGroup sync.WaitGroup

	r := NewHTTPReceiver(exit, &exitGroup)
	c := NewConcentrator(concentratorBucketSize, concentratorStrategy, concentratorGKEpsilon, exit, &exitGroup)
	spW := NewSpanWriter(APIEndpoint, exit, &exitGroup)
	stW := NewStatsWriter(APIEndpoint, exit, &exitGroup)

	return &Agent{
		Receiver:     r,
		Concentrator: c,
		SpanWriter:   spW,
		StatsWriter:  stW,
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
	a.statsFromConcentrator = make(chan model.StatsBucket)
	a.Concentrator.Init(a.spansToConcentrator, a.statsFromConcentrator)

	// Writer initialization
	a.spansToWriter = make(chan model.Span)
	a.SpanWriter.Init(a.spansToWriter)
	a.StatsWriter.Init(a.statsFromConcentrator)

	// FIXME: catch initialization errors
	return nil
}

// Start starts routers routines and individual pieces forever
func (a *Agent) Start() error {
	log.Info("Starting agent")

	// Build the pipeline in the opposite way the data is processed
	a.SpanWriter.Start()
	a.StatsWriter.Start()

	// sends stuff to the stats writer
	a.Concentrator.Start()

	// send stuff to the concentrator and span writer
	go func() {
		// should be closed by the listener when exiting
		for s := range a.spansFromReceiver {
			a.spansToWriter <- s
			a.spansToConcentrator <- s
		}
		// when no more spans are to be read from the receiver, notify downstream readers
		close(a.spansToWriter)
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

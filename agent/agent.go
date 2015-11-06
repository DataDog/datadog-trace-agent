package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/DataDog/raclette/config"
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
}

func GetQuantilesFromConfig(conf *config.File) ([]float64, error) {
	confQuantiles, err := conf.GetStrArray("trace.concentrator", "quantiles", ",")

	if err != nil {
		return nil, err
	}

	quantiles := make([]float64, len(confQuantiles))

	for index, q := range confQuantiles {
		value, err := strconv.ParseFloat(q, 64)
		if err != nil {
			return nil, err
		}
		quantiles[index] = value
	}
	return quantiles, nil
}

// NewAgent returns a new Agent object, ready to be initialized and started
func NewAgent(conf *config.File) *Agent {

	exit := make(chan struct{})
	var exitGroup sync.WaitGroup

	r, rawSpans := NewHTTPReceiver(exit, &exitGroup)
	q, quantizedSpans := NewQuantizer(rawSpans, exit, &exitGroup)

	extraAggr, err := conf.GetStrArray("trace.concentrator", "extra_aggregators", ",")
	if err != nil {
		log.Info("No aggregator configuration, using defaults")
	}

	bucketSize := conf.GetIntDefault("trace.concentrator", "bucket_size_seconds", 10)
	bucketQuantiles, err := GetQuantilesFromConfig(conf)

	// fail if quantiles configuration missing
	if err != nil {
		panic(err)
	}

	c, concentratedBuckets := NewConcentrator(time.Duration(bucketSize)*time.Second, quantizedSpans, extraAggr, exit, &exitGroup, bucketQuantiles)

	var endpoint BucketEndpoint
	if conf.GetBool("trace.api", "enabled", true) {
		apiKey, err := conf.Get("trace.api", "api_key")
		if err != nil {
			panic(err)
		}
		url := conf.GetDefault("trace.api", "endpoint", "http://localhost:8012/api/v0.1")
		endpoint = NewAPIEndpoint(url, apiKey)
	} else {
		log.Info("using null endpoint")
		endpoint = NullEndpoint{}
	}

	w := NewWriter(endpoint, concentratedBuckets, exit, &exitGroup)

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
	a.Concentrator.Start()
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

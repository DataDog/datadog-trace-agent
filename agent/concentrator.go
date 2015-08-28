// Concentrator
// https://en.wikipedia.org/wiki/Knelson_concentrator
// Gets an imperial shitton of traces, and outputs pre-computed data structures
// allowing to find the gold (stats) amongst the traces.
package main

import (
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// Concentrator produces time bucketed statistics from a stream of raw traces.
type Concentrator struct {
	// work channels
	inSpans  chan model.Span        // incoming spans to add to stats
	outStats chan model.StatsBucket // outgoing stats buckets
	outSpans chan model.Span        // spans that potentially need to be written with that time bucket

	// exit channels
	exit      chan bool
	exitGroup *sync.WaitGroup

	bucket time.Duration // the duration of our stats windows
	eps    float64       // quantile precision between 0 and 1

	// internal data structs
	openBucket    [2]model.StatsBucket
	currentBucket int32
}

// NewConcentrator returns that collects stats for every bucket.
func NewConcentrator(bucket time.Duration, eps float64, exit chan bool, exitGroup *sync.WaitGroup) *Concentrator {

	log.Infof("Starting new concentrator bucket:%s eps:%s", bucket, eps)
	return &Concentrator{
		bucket:    bucket,
		eps:       eps,
		exit:      exit,
		exitGroup: exitGroup,
	}
}

// Init sets the channels for incoming spans and outgoing stats before starting
func (c *Concentrator) Init(inSpans chan model.Span, outStats chan model.StatsBucket, outSpans chan model.Span) {
	c.inSpans = inSpans
	c.outStats = outStats
	c.outSpans = outSpans
}

// Start initializes the first structures and starts consuming stuff
func (c *Concentrator) Start() {
	// First bucket needs to be initialized manually now
	c.openBucket[0] = model.NewStatsBucket(c.eps)

	go func() {
		// should return when upstream span channel is closed
		for s := range c.inSpans {
			c.HandleNewSpan(s)
			c.outSpans <- s
		}
	}()

	go c.closeBuckets()

	log.Info("Concentrator started")
}

// HandleNewSpan adds to the current bucket the pointed span
func (c *Concentrator) HandleNewSpan(s model.Span) {
	c.openBucket[c.currentBucket].HandleSpan(s)
}

func (c *Concentrator) flush() {
	nextBucket := (c.currentBucket + 1) % 2
	c.openBucket[nextBucket] = model.NewStatsBucket(c.eps)

	//FIXME: use a mutex? too slow? don't care about potential traces written to previous bucket?
	// Use it and close the previous one
	c.openBucket[c.currentBucket].Duration = model.Now() - c.openBucket[c.currentBucket].Start
	c.currentBucket = nextBucket

	// flush the other bucket before
	bucketToSend := (c.currentBucket + 1) % 2
	if c.openBucket[bucketToSend].Start != 0 {
		// prepare for serialization
		c.openBucket[bucketToSend].Encode()
		c.outStats <- c.openBucket[bucketToSend]
	}
}

func (c *Concentrator) closeBuckets() {
	// block on the closer, to flush cleanly last bucket
	c.exitGroup.Add(1)
	ticker := time.Tick(c.bucket)
	for {
		select {
		case <-c.exit:
			log.Info("Concentrator exiting")
			// FIXME: don't flush, because downstream the writer is already shutting down
			// c.flush()

			// return cleanly and close writer chans
			close(c.outSpans)
			c.exitGroup.Done()
			return
		case <-ticker:
			log.Info("Concentrator flushed a time bucket")
			c.flush()
		}
	}
}

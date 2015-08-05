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

// Concentrator is getting a stream of raw traces and producing some time-bucketed normalized statistics from them.
//  * incSpans, channel from which we consume spans and create stats
//  * outStats, channel where we return our computed stats
//	* bucketDuration, designates the length of a time bucket
//	* openBucket, array of stats buckets we keep in memory (fixed size and iterating over)
//  * currentBucket, the index of openBucket we're currently writing to
type Concentrator struct {
	// work channels
	incSpans chan model.Span
	outStats chan model.StatsBucket

	// exit channels
	exit      chan bool
	exitGroup *sync.WaitGroup

	// configuration
	bucketDuration int32
	strategy       model.Strategy
	gkEps          float64

	// internal data structs
	openBucket    [2]*model.StatsBucket
	currentBucket int32
}

// NewConcentrator yields a new Concentrator flushing at a bucketDuration secondspace
func NewConcentrator(bucketDuration int32, strategy model.Strategy, gkEps float64, exit chan bool, exitGroup *sync.WaitGroup) *Concentrator {
	return &Concentrator{
		bucketDuration: bucketDuration,
		strategy:       strategy,
		gkEps:          gkEps,
		exit:           exit,
		exitGroup:      exitGroup,
	}
}

// Init sets the channels for incoming spans and outgoing stats before starting
func (c *Concentrator) Init(incSpans chan model.Span, outStats chan model.StatsBucket) {
	c.incSpans = incSpans
	c.outStats = outStats
}

// Start initializes the first structures and starts consuming stuff
func (c *Concentrator) Start() {
	// First bucket needs to be initialized manually now
	c.openBucket[0] = model.NewStatsBucket(c.strategy, c.gkEps)

	go func() {
		// should return when upstream span channel is closed
		for s := range c.incSpans {
			c.HandleNewSpan(&s)
		}
	}()

	go c.bucketCloser()

	log.Info("Concentrator started")
}

// HandleNewSpan adds to the current bucket the pointed span
func (c *Concentrator) HandleNewSpan(s *model.Span) {
	c.openBucket[c.currentBucket].HandleSpan(s)
}

func (c *Concentrator) flush() {
	nextBucket := (c.currentBucket + 1) % 2
	c.openBucket[nextBucket] = model.NewStatsBucket(c.strategy, c.gkEps)

	//FIXME: use a mutex? too slow? don't care about potential traces written to previous bucket?
	// Use it and close the previous one
	c.openBucket[c.currentBucket].End = model.Now()
	c.currentBucket = nextBucket

	// flush the other bucket before
	bucketToSend := (c.currentBucket + 1) % 2
	if c.openBucket[bucketToSend] != nil {
		c.outStats <- *c.openBucket[bucketToSend]
	}
}

func (c *Concentrator) bucketCloser() {
	// block on the closer, to flush cleanly last bucket
	c.exitGroup.Add(1)
	ticker := time.Tick(time.Duration(c.bucketDuration) * time.Second)
	for {
		select {
		case <-c.exit:
			log.Info("Concentrator received exit signal, flushing current bucket and exiting")
			c.flush()

			// return cleanly
			c.exitGroup.Done()
			return
		case <-ticker:
			log.Info("Concentrator closing & flushing another stats bucket")
			c.flush()
		}
	}
}

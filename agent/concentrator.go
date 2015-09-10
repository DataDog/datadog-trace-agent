package main

import (
	"errors"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// Discard spans that are older than this value in the concentrator (nanoseconds)
var OldestSpanCutoff = time.Duration(5 * time.Second).Nanoseconds()

// Concentrator produces time bucketed statistics from a stream of raw traces.
// https://en.wikipedia.org/wiki/Knelson_concentrator
// Gets an imperial shitton of traces, and outputs pre-computed data structures
// allowing to find the gold (stats) amongst the traces.
type Concentrator struct {
	inSpans          chan model.Span             // incoming spans to process
	outSpans         chan model.Span             // pass-thru channel for spans after having been concentrated
	outStats         chan model.StatsBucket      // outgoing stats buckets
	bucketSize       time.Duration               // the size of our pre-aggregation per bucket
	buckets          map[int64]model.StatsBucket // buckets use to aggregate stats per timestamp
	lock             sync.Mutex                  // lock to read/write buckets
	oldestSpanCutoff int64                       // maximum time we wait before discarding straggling spans

	// exit channels used for synchronisation and sending stop signals
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// NewConcentrator initializes a new concentrator ready to be started and aggregate stats
func NewConcentrator(bucketSize time.Duration, inSpans chan model.Span, exit chan struct{}, exitGroup *sync.WaitGroup) (*Concentrator, chan model.Span, chan model.StatsBucket) {
	c := Concentrator{
		inSpans:          inSpans,
		outSpans:         make(chan model.Span),
		outStats:         make(chan model.StatsBucket),
		bucketSize:       bucketSize,
		buckets:          make(map[int64]model.StatsBucket),
		exit:             exit,
		oldestSpanCutoff: OldestSpanCutoff, // set by default, useful to override to have faster tests
		exitGroup:        exitGroup,
	}

	return &c, c.outSpans, c.outStats
}

// Start initializes the first structures and starts consuming stuff
func (c *Concentrator) Start() {
	c.exitGroup.Add(1)

	go func() {
		// should return when upstream span channel is closed
		for s := range c.inSpans {
			err := c.HandleNewSpan(s)
			if err == nil {
				c.outSpans <- s
			} else {
				log.Debugf("Span rejected by concentrator. Reason: %v", err)
			}
		}
	}()

	go c.closeBuckets()

	log.Info("Concentrator started")
}

// HandleNewSpan adds to the current bucket the pointed span
func (c *Concentrator) HandleNewSpan(s model.Span) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	bucket := s.Start - s.Start%c.bucketSize.Nanoseconds()
	if model.Now()-bucket > OldestSpanCutoff {
		return errors.New("Late span rejected")
	}

	b, ok := c.buckets[bucket]
	if !ok {
		b = model.NewStatsBucket(bucket, c.bucketSize.Nanoseconds())
		c.buckets[bucket] = b
	}

	b.HandleSpan(s)
	return nil
}

func (c *Concentrator) flush() {
	c.lock.Lock()
	defer c.lock.Unlock()

	now := model.Now()
	lastBucket := now - now%c.bucketSize.Nanoseconds()

	for bucket, stats := range c.buckets {
		// flush & expire old buckets that cannot be hit anymore
		if bucket < now-c.oldestSpanCutoff && bucket != lastBucket {
			log.Infof("Concentrator flushed time bucket %d", bucket)
			c.outStats <- stats
			delete(c.buckets, bucket)
		}
	}
}

func (c *Concentrator) closeBuckets() {
	// block on the closer, to flush cleanly last bucket
	ticker := time.Tick(c.bucketSize)
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
			c.flush()
		}
	}
}

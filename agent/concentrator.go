package main

import (
	"errors"
	"expvar"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

var (
	eLateSpans = expvar.NewInt("LateSpans")
)

// By default the finest grain we aggregate to
var DefaultAggregators = []string{"service", "resource"}

// ConcentratorBucket is what the concentrator produces: bucketed spans/summary stats
type ConcentratorBucket struct {
	Sampler Sampler
	Stats   model.StatsBucket
}

func newBucket(ts, d int64) ConcentratorBucket {
	return ConcentratorBucket{
		Stats:   model.NewStatsBucket(ts, d),
		Sampler: NewSampler(),
	}
}

func (cb ConcentratorBucket) handleSpan(s model.Span, aggr []string) {
	cb.Stats.HandleSpan(s, aggr)
	cb.Sampler.AddSpan(s)
}

func (cb ConcentratorBucket) isEmpty() bool {
	return cb.Sampler.IsEmpty() && cb.Stats.IsEmpty()
}

func (cb ConcentratorBucket) buildPayload() model.AgentPayload {
	return model.AgentPayload{
		Spans: cb.Sampler.GetSamples(cb.Stats),
		Stats: cb.Stats,
	}
}

// Concentrator produces time bucketed statistics from a stream of raw traces.
// https://en.wikipedia.org/wiki/Knelson_concentrator
// Gets an imperial shitton of traces, and outputs pre-computed data structures
// allowing to find the gold (stats) amongst the traces.
// It also takes care of inserting the spans in a sampler.
type Concentrator struct {
	inSpans     chan model.Span              // incoming spans to process
	outBuckets  chan ConcentratorBucket      // outgoing buckets
	buckets     map[int64]ConcentratorBucket // buckets use to aggregate stats per timestamp
	aggregators []string                     // we'll always aggregate (if possible) to this finest grain
	lock        sync.Mutex                   // lock to read/write buckets

	conf *config.AgentConfig

	// exit channels used for synchronisation and sending stop signals
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// NewConcentrator initializes a new concentrator ready to be started and aggregate stats
func NewConcentrator(
	inSpans chan model.Span, conf *config.AgentConfig, exit chan struct{}, exitGroup *sync.WaitGroup,
) (*Concentrator, chan ConcentratorBucket) {
	c := Concentrator{
		inSpans:     inSpans,
		outBuckets:  make(chan ConcentratorBucket),
		buckets:     make(map[int64]ConcentratorBucket),
		aggregators: append(DefaultAggregators, conf.ExtraAggregators...),
		conf:        conf,
		exit:        exit,
		exitGroup:   exitGroup,
	}

	return &c, c.outBuckets
}

// Start initializes the first structures and starts consuming stuff
func (c *Concentrator) Start() {
	c.exitGroup.Add(1)

	go func() {
		// should return when upstream span channel is closed
		for s := range c.inSpans {
			err := c.HandleNewSpan(s)
			if err != nil {
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

	bucketTs := s.Start - s.Start%c.conf.BucketInterval.Nanoseconds()

	// TODO[leo]: figure out what's the best strategy here
	if model.Now()-bucketTs > c.conf.OldestSpanCutoff {
		eLateSpans.Add(1)
		return errors.New("Late span rejected")
	}

	b, ok := c.buckets[bucketTs]
	if !ok {
		b = newBucket(bucketTs, c.conf.BucketInterval.Nanoseconds())
		c.buckets[bucketTs] = b
	}

	b.handleSpan(s, c.aggregators)
	return nil
}

func (c *Concentrator) flush() {
	c.lock.Lock()
	defer c.lock.Unlock()

	now := model.Now()
	lastBucketTs := now - now%c.conf.BucketInterval.Nanoseconds()

	for ts, bucket := range c.buckets {
		// flush & expire old buckets that cannot be hit anymore
		if ts < now-c.conf.OldestSpanCutoff && ts != lastBucketTs {
			log.Infof("Concentrator flushed bucket %d", ts)
			c.outBuckets <- bucket
			delete(c.buckets, ts)
		}
	}
}

func (c *Concentrator) closeBuckets() {
	// block on the closer, to flush cleanly last bucket
	ticker := time.Tick(c.conf.BucketInterval)
	for {
		select {
		case <-c.exit:
			log.Info("Concentrator exiting")
			// FIXME: don't flush, because downstream the writer is already shutting down
			// c.flush()

			// return cleanly and close writer chans
			close(c.outBuckets)
			c.exitGroup.Done()
			return
		case <-ticker:
			c.flush()
		}
	}
}

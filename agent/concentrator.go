package main

import (
	"sort"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// Concentrator produces time bucketed statistics from a stream of raw traces.
// https://en.wikipedia.org/wiki/Knelson_concentrator
// Gets an imperial shitton of traces, and outputs pre-computed data structures
// allowing to find the gold (stats) amongst the traces.
type Concentrator struct {
	// list of attributes to use for extra aggregation
	aggregators []string
	// bucket duration in nanoseconds
	bsize int64

	OutStats chan []model.StatsBucket

	exit   chan struct{}
	exitWG *sync.WaitGroup

	buckets map[int64]*model.StatsRawBucket // buckets used to aggregate stats per timestamp
	mu      sync.Mutex
}

// NewConcentrator initializes a new concentrator ready to be started
func NewConcentrator(aggregators []string, bsize int64, out chan []model.StatsBucket) *Concentrator {
	c := Concentrator{
		aggregators: aggregators,
		bsize:       bsize,
		buckets:     make(map[int64]*model.StatsRawBucket),

		OutStats: out,

		exit:   make(chan struct{}),
		exitWG: &sync.WaitGroup{},
	}
	sort.Strings(c.aggregators)
	return &c
}

// Start starts the concentrator.
func (c *Concentrator) Start() {
	go func() {
		defer watchdog.LogOnPanic()
		c.Run()
	}()
}

// Run runs the main loop of the concentrator goroutine. Traces are received
// through `Add`, this loop only deals with flushing.
func (c *Concentrator) Run() {
	c.exitWG.Add(1)
	defer c.exitWG.Done()

	// flush with the same period as stats buckets
	flushTicker := time.NewTicker(time.Duration(c.bsize) * time.Nanosecond)
	defer flushTicker.Stop()

	log.Debug("starting concentrator")

	for {
		select {
		case <-flushTicker.C:
			c.OutStats <- c.Flush()
		case <-c.exit:
			log.Info("exiting concentrator, computing remaining stats")
			c.OutStats <- c.Flush()
			return
		}
	}
}

// Stop stops the main Run loop.
func (c *Concentrator) Stop() {
	close(c.exit)
	c.exitWG.Wait()
}

// Add appends to the proper stats bucket this trace's statistics
func (c *Concentrator) Add(t processedTrace) {
	c.mu.Lock()

	for _, s := range t.WeightedTrace {
		btime := s.End() - s.End()%c.bsize
		b, ok := c.buckets[btime]
		if !ok {
			b = model.NewStatsRawBucket(btime, c.bsize)
			c.buckets[btime] = b
		}

		if t.Root != nil && s.SpanID == t.Root.SpanID && t.Sublayers != nil {
			// handle sublayers
			b.HandleSpan(s, t.Env, c.aggregators, &t.Sublayers)
		} else {
			b.HandleSpan(s, t.Env, c.aggregators, nil)
		}
	}

	c.mu.Unlock()
}

// Flush deletes and returns complete statistic buckets
func (c *Concentrator) Flush() []model.StatsBucket {
	var sb []model.StatsBucket
	now := model.Now()

	c.mu.Lock()
	for ts, srb := range c.buckets {
		bucket := srb.Export()

		// always keep one bucket opened
		// this is a trade-off: we accept slightly late traces (clock skew and stuff)
		// but we delay flushing by at most 2 buckets
		if ts > now-2*c.bsize {
			continue
		}

		log.Debugf("flushing bucket %d", ts)
		for _, d := range bucket.Distributions {
			statsd.Client.Histogram("datadog.trace_agent.distribution.len", float64(d.Summary.N), nil, 1)
		}
		for _, d := range bucket.ErrDistributions {
			statsd.Client.Histogram("datadog.trace_agent.err_distribution.len", float64(d.Summary.N), nil, 1)
		}
		sb = append(sb, bucket)
		delete(c.buckets, ts)
	}
	c.mu.Unlock()

	return sb
}

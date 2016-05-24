package main

import (
	"errors"
	"expvar"
	"sort"
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

var (
	eLateSpans = expvar.NewInt("LateSpans")
)

// DefaultAggregators are the finest grain we aggregate to by default
var DefaultAggregators = []string{"service", "name", "resource"}

// Concentrator produces time bucketed statistics from a stream of raw traces.
// https://en.wikipedia.org/wiki/Knelson_concentrator
// Gets an imperial shitton of traces, and outputs pre-computed data structures
// allowing to find the gold (stats) amongst the traces.
// It also takes care of inserting the spans in a sampler.
type Concentrator struct {
	in          chan model.Trace            // incoming spans to process
	out         chan []model.StatsBucket    // outgoing payload
	buckets     map[int64]model.StatsBucket // buckets use to aggregate stats per timestamp
	aggregators []string                    // we'll always aggregate (if possible) to this finest grain
	lock        sync.Mutex                  // lock to read/write buckets

	conf *config.AgentConfig

	Worker
}

// NewConcentrator initializes a new concentrator ready to be started
func NewConcentrator(in chan model.Trace, conf *config.AgentConfig) *Concentrator {
	c := &Concentrator{
		in:          in,
		out:         make(chan []model.StatsBucket),
		buckets:     make(map[int64]model.StatsBucket),
		aggregators: append(DefaultAggregators, conf.ExtraAggregators...),
		conf:        conf,
	}
	c.Init()
	return c
}

// Start initializes the first structures and starts consuming spans
func (c *Concentrator) Start() {
	c.wg.Add(1)

	go func() {
		for {
			select {
			case t := <-c.in:
				if len(t) == 1 && t[0].IsFlushMarker() {
					log.Debug("Concentrator starts a flush")
					c.out <- c.Flush()
				} else {
					for _, s := range t {
						err := c.HandleNewSpan(s)
						if err != nil {
							log.Debugf("Span %v rejected by concentrator. Reason: %v", s.SpanID, err)
						}
					}
				}
			case <-c.exit:
				log.Info("Concentrator exiting")
				close(c.out)
				c.wg.Done()
				return
			}
		}
	}()

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
		b = model.NewStatsBucket(
			bucketTs, c.conf.BucketInterval.Nanoseconds(), c.conf.LatencyResolution,
		)
		c.buckets[bucketTs] = b
	}

	b.HandleSpan(s, c.aggregators)
	return nil
}

// Int64Slice attaches the methods of sort.Interface to []int64.
type Int64Slice []int64

func (p Int64Slice) Len() int           { return len(p) }
func (p Int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func sortInts64(a []int64)              { sort.Sort(Int64Slice(a)) }

// Flush deletes and returns complete statistic buckets
func (c *Concentrator) Flush() []model.StatsBucket {
	now := model.Now()
	lastBucketTs := now - now%c.conf.BucketInterval.Nanoseconds()
	sb := []model.StatsBucket{}
	keys := []int64{}

	c.lock.Lock()
	defer c.lock.Unlock()

	// Sort buckets by timestamp
	for k := range c.buckets {
		keys = append(keys, k)
	}
	sortInts64(keys)

	for i := range keys {
		ts := keys[i]
		bucket := c.buckets[ts]
		// flush & expire old buckets that cannot be hit anymore
		if ts < now-c.conf.OldestSpanCutoff && ts != lastBucketTs {
			log.Infof("Concentrator adds bucket to payload %d", ts)
			sb = append(sb, bucket)
			delete(c.buckets, ts)
		}
	}
	log.Debugf("Concentrator flushes %d stats buckets", len(sb))
	return sb
}

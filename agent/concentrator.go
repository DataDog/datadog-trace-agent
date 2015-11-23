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

// By default the finest grain we aggregate to
var DefaultAggregators = []string{"service", "resource"}

// Concentrator produces time bucketed statistics from a stream of raw traces.
// https://en.wikipedia.org/wiki/Knelson_concentrator
// Gets an imperial shitton of traces, and outputs pre-computed data structures
// allowing to find the gold (stats) amongst the traces.
// It also takes care of inserting the spans in a sampler.
type Concentrator struct {
	in          chan model.Span             // incoming spans to process
	out         chan []model.StatsBucket    // outgoing payload
	buckets     map[int64]model.StatsBucket // buckets use to aggregate stats per timestamp
	aggregators []string                    // we'll always aggregate (if possible) to this finest grain
	lock        sync.Mutex                  // lock to read/write buckets

	conf *config.AgentConfig

	Worker
}

// NewConcentrator initializes a new concentrator ready to be started and aggregate stats
func NewConcentrator(
	in chan model.Span, conf *config.AgentConfig,
) *Concentrator {
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

// Start initializes the first structures and starts consuming stuff
func (c *Concentrator) Start() {
	c.wg.Add(1)

	go func() {
		// should return when upstream span channel is closed
		for s := range c.in {
			if s.IsFlushMarker() {
				log.Debug("Concentrator starts a flush")
				c.out <- c.Flush()
			} else {
				err := c.HandleNewSpan(s)
				if err != nil {
					log.Debugf("Span %v rejected by concentrator. Reason: %v", s.SpanID, err)
				}
			}
		}
	}()

	go func() {
		<-c.exit
		log.Info("Concentrator exiting")
		close(c.out)
		c.wg.Done()
		return
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
		b = model.NewStatsBucket(bucketTs, c.conf.BucketInterval.Nanoseconds())
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

func SortInts64(a []int64) {
	sort.Sort(Int64Slice(a))
}

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
	SortInts64(keys)

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

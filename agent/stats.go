package main

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/DataDog/datadog-trace-agent/statsd"
)

type receiverStats struct {
	sync.RWMutex
	Stats map[uint64]*tagStats
}

func newReceiverStats() *receiverStats {
	return &receiverStats{sync.RWMutex{}, map[uint64]*tagStats{}}
}

func (rs *receiverStats) getTagStats(tags []string) *tagStats {
	hash := hash(tags)

	rs.Lock()
	tagStats, ok := rs.Stats[hash]
	if !ok {
		tagStats = newTagStats(tags)
		rs.Stats[hash] = tagStats
	}
	rs.Unlock()

	return tagStats
}

func (rs *receiverStats) update(ts *tagStats) {
	rs.Lock()
	tagStats, ok := rs.Stats[ts.Hash]
	if !ok {
		rs.Stats[ts.Hash] = ts
	} else {
		tagStats.update(ts.Stats)
	}
	rs.Unlock()
}

func (rs *receiverStats) acc(new *receiverStats) {
	new.Lock()
	for _, tagStats := range new.Stats {
		ts := rs.getTagStats(tagStats.Tags)
		ts.update(tagStats.Stats)
	}
	new.Unlock()
}

func (rs *receiverStats) publish() {
	rs.RLock()
	for _, tagStats := range rs.Stats {
		tagStats.publish()
	}
	rs.RUnlock()
}

func (rs *receiverStats) tot() Stats {
	tot := Stats{}
	rs.RLock()
	for _, tagStats := range rs.Stats {
		tot.update(tagStats.Stats)
	}
	rs.RUnlock()
	return tot
}

func (rs *receiverStats) reset() {
	rs.Lock()
	for _, tagStats := range rs.Stats {
		tagStats.reset()
	}
	rs.Unlock()
}

func (rs *receiverStats) clone() *receiverStats {
	clone := newReceiverStats()
	clone.acc(rs)
	return clone
}

func (rs *receiverStats) String() string {
	str := ""
	rs.RLock()
	for _, tagStats := range rs.Stats {
		str += tagStats.String()
	}
	rs.RUnlock()
	return str
}

type tagStats struct {
	Stats
	Tags []string
	Hash uint64
}

func newTagStats(tags []string) *tagStats {
	if tags == nil {
		tags = []string{}
	}
	return &tagStats{Stats{}, tags, hash(tags)}
}

func (ts *tagStats) publish() {
	// Atomically load the stats from ts
	tracesReceived := atomic.LoadInt64(&ts.TracesReceived)
	tracesDropped := atomic.LoadInt64(&ts.TracesDropped)
	tracesBytes := atomic.LoadInt64(&ts.TracesBytes)
	spansReceived := atomic.LoadInt64(&ts.SpansReceived)
	spansDropped := atomic.LoadInt64(&ts.SpansDropped)
	servicesBytes := atomic.LoadInt64(&ts.ServicesBytes)
	servicesMeta := atomic.LoadInt64(&ts.ServicesMeta)

	// Publish the stats
	statsd.Client.Count("datadog.trace_agent.receiver.traces_received", tracesReceived, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.traces_dropped", tracesDropped, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.traces_bytes", tracesBytes, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.spans_received", spansReceived, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.spans_dropped", spansDropped, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.services_bytes", servicesBytes, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.services_meta", servicesMeta, ts.Tags, 1)
}

func (ts *tagStats) clone() *tagStats {
	return &tagStats{ts.Stats, ts.Tags, ts.Hash}
}

func (ts *tagStats) String() string {
	return fmt.Sprintf("\n\t%v -> traces received: %v, traces dropped: %v", ts.Tags, ts.TracesReceived, ts.TracesDropped)
}

// Stats holds the metrics that will be reported every 10s by the agent.
// Its fields require to be accessed in an atomic way.
type Stats struct {
	// TracesReceived is the total number of traces received, including the dropped ones
	TracesReceived int64
	// TracesDropped is the number of traces dropped
	TracesDropped int64
	// TracesBytes is the amount of data received on the traces endpoint (raw data, encoded, compressed)
	TracesBytes int64
	// SpansReceived is the total number of spans received, including the dropped ones
	SpansReceived int64
	// SpansDropped is the number of spans dropped
	SpansDropped int64
	// ServicesBytes is the amount of data received on the services endpoint (raw data, encoded, compressed)
	ServicesBytes int64
	// ServicesMeta is the size of the services meta data
	ServicesMeta int64
}

func (s *Stats) update(new Stats) {
	atomic.AddInt64(&s.TracesReceived, new.TracesReceived)
	atomic.AddInt64(&s.TracesDropped, new.TracesDropped)
	atomic.AddInt64(&s.TracesBytes, new.TracesBytes)
	atomic.AddInt64(&s.SpansReceived, new.SpansReceived)
	atomic.AddInt64(&s.SpansDropped, new.SpansDropped)
	atomic.AddInt64(&s.ServicesBytes, new.ServicesBytes)
	atomic.AddInt64(&s.ServicesMeta, new.ServicesMeta)
}

func (s *Stats) reset() {
	atomic.StoreInt64(&s.TracesReceived, 0)
	atomic.StoreInt64(&s.TracesDropped, 0)
	atomic.StoreInt64(&s.TracesBytes, 0)
	atomic.StoreInt64(&s.SpansReceived, 0)
	atomic.StoreInt64(&s.SpansDropped, 0)
	atomic.StoreInt64(&s.ServicesBytes, 0)
	atomic.StoreInt64(&s.ServicesMeta, 0)
}

// hash returns the hash of the tag slice
func hash(tags []string) uint64 {
	h := fnv.New64()
	s := strings.Join(tags, "")
	h.Write([]byte(s))
	return h.Sum64()
}

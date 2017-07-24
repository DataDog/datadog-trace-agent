package main

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/DataDog/datadog-trace-agent/statsd"
)

// receiverStats is used to store all the stats per tags.
type receiverStats struct {
	sync.RWMutex
	Stats map[Tags]*tagStats
}

func newReceiverStats() *receiverStats {
	return &receiverStats{sync.RWMutex{}, map[Tags]*tagStats{}}
}

// getTagStats returns the struct in which the stats will be stored depending of their tags.
func (rs *receiverStats) getTagStats(tags Tags) *tagStats {
	rs.Lock()
	tagStats, ok := rs.Stats[tags]
	if !ok {
		tagStats = newTagStats(tags)
		rs.Stats[tags] = tagStats
	}
	rs.Unlock()

	return tagStats
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

// String gives a string representation of the receiverStats struct.
func (rs *receiverStats) String() string {
	str := ""
	rs.RLock()
	for _, ts := range rs.Stats {
		str += fmt.Sprintf("\n\t%v -> %s", ts.Tags, ts.String())

	}
	rs.RUnlock()
	return str
}

// tagStats is the struct used to associate the stats with their set of tags.
type tagStats struct {
	Tags
	Stats
}

func newTagStats(tags Tags) *tagStats {
	return &tagStats{tags, Stats{}}
}

func (ts *tagStats) publish() {
	// Atomically load the stats from ts
	tracesReceived := atomic.LoadInt64(&ts.TracesReceived)
	tracesDropped := atomic.LoadInt64(&ts.TracesDropped)
	tracesBytes := atomic.LoadInt64(&ts.TracesBytes)
	spansReceived := atomic.LoadInt64(&ts.SpansReceived)
	spansDropped := atomic.LoadInt64(&ts.SpansDropped)
	servicesReceived := atomic.LoadInt64(&ts.ServicesReceived)
	servicesBytes := atomic.LoadInt64(&ts.ServicesBytes)

	// Publish the stats
	statsd.Client.Count("datadog.trace_agent.receiver.traces_received", tracesReceived, ts.Tags.toArray(), 1)
	statsd.Client.Count("datadog.trace_agent.receiver.traces_dropped", tracesDropped, ts.Tags.toArray(), 1)
	statsd.Client.Count("datadog.trace_agent.receiver.traces_bytes", tracesBytes, ts.Tags.toArray(), 1)
	statsd.Client.Count("datadog.trace_agent.receiver.spans_received", spansReceived, ts.Tags.toArray(), 1)
	statsd.Client.Count("datadog.trace_agent.receiver.spans_dropped", spansDropped, ts.Tags.toArray(), 1)
	statsd.Client.Count("datadog.trace_agent.receiver.services_received", servicesReceived, ts.Tags.toArray(), 1)
	statsd.Client.Count("datadog.trace_agent.receiver.services_bytes", servicesBytes, ts.Tags.toArray(), 1)
}

func (ts *tagStats) clone() *tagStats {
	return &tagStats{ts.Tags, ts.Stats}
}

// Stats holds the metrics that will be reported every 10s by the agent.
// Its fields require to be accessed in an atomic way.
type Stats struct {
	// TracesReceived is the total number of traces received, including the dropped ones.
	TracesReceived int64
	// TracesDropped is the number of traces dropped.
	TracesDropped int64
	// TracesBytes is the amount of data received on the traces endpoint (raw data, encoded, compressed).
	TracesBytes int64
	// SpansReceived is the total number of spans received, including the dropped ones.
	SpansReceived int64
	// SpansDropped is the number of spans dropped.
	SpansDropped int64
	// ServicesReceived is the number of services received.
	ServicesReceived int64
	// ServicesBytes is the amount of data received on the services endpoint (raw data, encoded, compressed).
	ServicesBytes int64
}

func (s *Stats) update(new Stats) {
	atomic.AddInt64(&s.TracesReceived, new.TracesReceived)
	atomic.AddInt64(&s.TracesDropped, new.TracesDropped)
	atomic.AddInt64(&s.TracesBytes, new.TracesBytes)
	atomic.AddInt64(&s.SpansReceived, new.SpansReceived)
	atomic.AddInt64(&s.SpansDropped, new.SpansDropped)
	atomic.AddInt64(&s.ServicesReceived, new.ServicesReceived)
	atomic.AddInt64(&s.ServicesBytes, new.ServicesBytes)
}

func (s *Stats) reset() {
	atomic.StoreInt64(&s.TracesReceived, 0)
	atomic.StoreInt64(&s.TracesDropped, 0)
	atomic.StoreInt64(&s.TracesBytes, 0)
	atomic.StoreInt64(&s.SpansReceived, 0)
	atomic.StoreInt64(&s.SpansDropped, 0)
	atomic.StoreInt64(&s.ServicesReceived, 0)
	atomic.StoreInt64(&s.ServicesBytes, 0)
}

// String returns a string representation of the Stats struct
func (s *Stats) String() string {
	// Atomically load the stas
	tracesReceived := atomic.LoadInt64(&s.TracesReceived)
	tracesDropped := atomic.LoadInt64(&s.TracesDropped)
	tracesBytes := atomic.LoadInt64(&s.TracesBytes)
	servicesReceived := atomic.LoadInt64(&s.ServicesReceived)
	servicesBytes := atomic.LoadInt64(&s.ServicesBytes)

	return fmt.Sprintf("traces received: %v, traces dropped: %v, amount of traces: %v bytes, services received: %v, amount of services: %v",
		tracesReceived, tracesDropped, tracesBytes, servicesReceived, servicesBytes)
}

// Tags holds the tags we parse when we handle the header of the payload.
type Tags struct {
	Lang, LangVersion, Interpreter, TracerVersion, Endpoint string
}

// toArray will transform the Tags struct into a slice of string.
// We only publish the non-empty tags.
func (t *Tags) toArray() []string {
	tags := make([]string, 0, 5)

	if t.Lang != "" {
		tags = append(tags, "lang:"+t.Lang)
	}
	if t.LangVersion != "" {
		tags = append(tags, "lang_version:"+t.LangVersion)
	}
	if t.Interpreter != "" {
		tags = append(tags, "interpreter:"+t.Interpreter)
	}
	if t.TracerVersion != "" {
		tags = append(tags, "tracer_version:"+t.TracerVersion)
	}
	if t.Endpoint != "" {
		tags = append(tags, "endpoint:"+t.Endpoint)
	}

	return tags
}

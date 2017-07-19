package main

import (
	"hash/fnv"
	"strings"
	"sync"

	"github.com/DataDog/datadog-trace-agent/statsd"
)

// headerFields is a map used to decode the header metas
var headerFields = map[string]string{
	"lang":           "Datadog-Meta-Lang",
	"lang_version":   "Datadog-Meta-Lang-Version",
	"interpreter":    "Datadog-Meta-Lang-Interpreter",
	"tracer_version": "Datadog-Meta-Tracer-Version",
}

type receiverStats struct {
	sync.RWMutex
	stats map[uint64]*tagStats
}

func newReceiverStats() *receiverStats {
	return &receiverStats{sync.RWMutex{}, map[uint64]*tagStats{}}
}

func (rs *receiverStats) update(ts *tagStats) {
	rs.Lock()
	if rs.stats[ts.hash] != nil {
		rs.stats[ts.hash].update(ts)
	} else {
		rs.stats[ts.hash] = ts
	}
	rs.Unlock()
}

func (rs *receiverStats) acc(new *receiverStats) {
	rs.Lock()
	new.RLock()
	for hash, tagStats := range new.stats {
		rs.stats[hash].update(tagStats)
	}
	new.RUnlock()
	rs.Unlock()
}

func (rs *receiverStats) publish() {
	rs.RLock()
	for _, tagStats := range rs.stats {
		tagStats.publish()
	}
	rs.RUnlock()
}

func (rs *receiverStats) reset() {
	rs.Lock()
	for _, tagStats := range rs.stats {
		tagStats.reset()
	}
	rs.Unlock()
}

type tagStats struct {
	stats
	tags []string
	hash uint64
}

type stats struct {
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

func newTagStats(tags []string) *tagStats {
	return &tagStats{stats{}, tags, hash(tags)}
}

func (ts *tagStats) update(new *tagStats) {
	ts.TracesReceived += new.TracesReceived
	ts.TracesDropped += new.TracesDropped
	ts.TracesBytes += new.TracesBytes
	ts.SpansReceived += new.SpansReceived
	ts.SpansDropped += new.SpansDropped
	ts.ServicesBytes += new.ServicesBytes
	ts.ServicesMeta += new.ServicesMeta
}

func (ts *tagStats) reset() {
	ts.TracesReceived = 0
	ts.TracesDropped = 0
	ts.TracesBytes = 0
	ts.SpansReceived = 0
	ts.SpansDropped = 0
	ts.ServicesBytes = 0
	ts.ServicesMeta = 0
}

func (ts *tagStats) publish() {
	statsd.Client.Count("datadog.trace_agent.receiver.traces_received", ts.TracesReceived, ts.tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.traces_dropped", ts.TracesDropped, ts.tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.traces_bytes", ts.TracesBytes, ts.tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.spans_received", ts.SpansReceived, ts.tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.spans_dropped", ts.SpansDropped, ts.tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.services_bytes", ts.ServicesBytes, ts.tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.services_meta", ts.ServicesMeta, ts.tags, 1)
}

// hash returns the hash of the tag slice
func hash(tags []string) uint64 {
	h := fnv.New64()
	s := strings.Join(tags, "")
	h.Write([]byte(s))
	return h.Sum64()
}

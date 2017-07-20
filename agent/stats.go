package main

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/DataDog/datadog-trace-agent/statsd"
)

type receiverStats struct {
	sync.RWMutex
	Stats map[uint64]*tagStats
}

func newReceiverStats() *receiverStats {
	return &receiverStats{sync.RWMutex{}, map[uint64]*tagStats{}}
}

func (rs *receiverStats) update(ts *tagStats) {
	rs.Lock()
	if rs.Stats[ts.Hash] != nil {
		rs.Stats[ts.Hash].update(ts.Stats)
	} else {
		rs.Stats[ts.Hash] = ts.clone()
	}
	rs.Unlock()
}

func (rs *receiverStats) acc(new *receiverStats) {
	new.RLock()
	for _, tagStats := range new.Stats {
		rs.update(tagStats)
	}
	new.RUnlock()
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
	statsd.Client.Count("datadog.trace_agent.receiver.traces_received", ts.TracesReceived, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.traces_dropped", ts.TracesDropped, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.traces_bytes", ts.TracesBytes, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.spans_received", ts.SpansReceived, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.spans_dropped", ts.SpansDropped, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.services_bytes", ts.ServicesBytes, ts.Tags, 1)
	statsd.Client.Count("datadog.trace_agent.receiver.services_meta", ts.ServicesMeta, ts.Tags, 1)
}

func (ts *tagStats) clone() *tagStats {
	return &tagStats{ts.Stats, ts.Tags, ts.Hash}
}

func (ts *tagStats) String() string {
	return fmt.Sprintf("\n\t%v -> traces received: %v, traces dropped: %v", ts.Tags, ts.TracesReceived, ts.TracesDropped)
}

// Stats holds the metrics that will be reported every 10s by the agent
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
	s.TracesReceived += new.TracesReceived
	s.TracesDropped += new.TracesDropped
	s.TracesBytes += new.TracesBytes
	s.SpansReceived += new.SpansReceived
	s.SpansDropped += new.SpansDropped
	s.ServicesBytes += new.ServicesBytes
	s.ServicesMeta += new.ServicesMeta
}

func (s *Stats) reset() {
	s.TracesReceived = 0
	s.TracesDropped = 0
	s.TracesBytes = 0
	s.SpansReceived = 0
	s.SpansDropped = 0
	s.ServicesBytes = 0
	s.ServicesMeta = 0
}

// hash returns the hash of the tag slice
func hash(tags []string) uint64 {
	h := fnv.New64()
	s := strings.Join(tags, "")
	h.Write([]byte(s))
	return h.Sum64()
}

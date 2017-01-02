package profile

import (
	"github.com/DataDog/datadog-trace-agent/statsd"
	"runtime"
)

// MemStatsd writes memory statistics to Datadog data pipeline using Statsd.
func MemStatsd(tags []string) {
	var ms runtime.MemStats

	runtime.ReadMemStats(&ms)
	statsd.Client.Count("trace_agent.profile.mem.total_alloc", int64(ms.TotalAlloc), tags, 1)
	statsd.Client.Count("trace_agent.profile.mem.sys", int64(ms.Sys), tags, 1)
	statsd.Client.Count("trace_agent.profile.mem.lookups", int64(ms.Lookups), tags, 1)
	statsd.Client.Count("trace_agent.profile.mem.mallocs", int64(ms.Mallocs), tags, 1)
	statsd.Client.Count("trace_agent.profile.mem.frees", int64(ms.Frees), tags, 1)
}

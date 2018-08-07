package collect

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/DataDog/datadog-trace-agent/internal/statsd"
)

// monitor runs a loop, sending occasional statsd entries.
func (c *Cache) monitor(client statsd.StatsClient) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			c.computeStats(client)
		}
	}
}

func (c *Cache) computeStats(client statsd.StatsClient) {
	iter := c.newReverseIterator()
	now := time.Now()
	var (
		n2m    float64
		n5m    float64
		nLarge float64
	)
	for {
		t, ok := iter.getAndAdvance()
		if !ok {
			break
		}
		idle := now.Sub(t.lastmod)
		switch {
		case idle <= 30*time.Second:
			break // stop counting, these are recent spans
		case idle < 2*time.Minute:
			n2m++
		case idle < 5*time.Minute:
			n5m++
		default:
			nLarge++
		}
	}

	total := float64(iter.len())
	client.Gauge("datadog.trace_agent.cache.traces", total-n2m-n5m-nLarge, []string{
		"version:v1",
		"idle:under30sec",
	}, 1)

	client.Gauge("datadog.trace_agent.cache.traces", n2m, []string{
		"version:v1",
		"idle:under2m",
	}, 1)

	client.Gauge("datadog.trace_agent.cache.traces", n5m, []string{
		"version:v1",
		"idle:under5m",
	}, 1)

	client.Gauge("datadog.trace_agent.cache.traces", nLarge, []string{
		"version:v1",
		"idle:over5m",
	}, 1)

	c.mu.RLock()
	client.Gauge("datadog.trace_agent.cache.bytes", float64(c.size), []string{"version:v1"}, 1)
	c.mu.RUnlock()
}

// ServeState is an http.HandlerFunc that will serve a snapshot of the
// state of the cache (in JSON) when accessed. It can only be accessed
// locally.
func (c *Cache) ServeState(w http.ResponseWriter, req *http.Request) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil || host != "127.0.0.1" {
		// do not allow this endpoint unless it's localhost
		w.WriteHeader(http.StatusNotFound)
		return
	}
	limit := 5
	if v := req.URL.Query().Get("l"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			limit = d
		}
	}
	mins := 5
	if v := req.URL.Query().Get("m"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			mins = d
		}
	}
	iter := c.newReverseIterator()
	now := time.Now()
	count := 0
	fmt.Fprint(w, "[")
	for {
		if count >= limit {
			break
		}
		t, ok := iter.getAndAdvance()
		if !ok {
			break
		}
		idle := now.Sub(t.lastmod)
		if idle < time.Duration(mins)*time.Minute {
			// reached recent traces
			break
		}
		if count > 0 {
			fmt.Fprint(w, ",")
		}
		// TODO: Make a structure and JSON-encode it
		fmt.Fprintf(w, `{
	"trace_id": %d, "age_min": %d, "total_spans": %d, "spans": [`, t.key, idle/time.Minute, len(t.spans))
		for i, span := range t.spans {
			if i > 2 {
				break
			}
			fmt.Fprintf(w,
				`
				{"name": %q, "service": %q, "resource": %q, "span_id": %d, "parent_id": %d}`,
				span.Name, span.Service, span.Resource, span.SpanID, span.ParentID,
			)
			switch n := len(t.spans); n {
			case 1:
				// no comma
			case 2:
				if i == 0 {
					fmt.Fprintf(w, ",")
				}
			default:
				if i < 2 {
					fmt.Fprintf(w, ",")
				}
			}
		}
		fmt.Fprint(w, "\n\t]\n}")
		count++
	}
	fmt.Fprint(w, "]\n")
}

package main

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/DataDog/datadog-trace-agent/test"
)

func TestHostname(t *testing.T) {
	var r test.Runner
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	t.Run("from-config", func(t *testing.T) {
		r.T = t
		if err := r.RunAgent([]byte(`hostname: asdq`)); err != nil {
			t.Fatal(err)
		}

		traceList := agent.Traces{
			agent.Trace{test.NewSpan(nil)},
			agent.Trace{test.NewSpan(nil)},
			agent.Trace{test.NewSpan(nil)},
			agent.Trace{test.NewSpan(nil)},
		}
		for i := 0; i < 10; i++ {
			go func() {
				if err := r.Post(traceList); err != nil {
					t.Fatal(err)
				}
			}()
		}

		for p := range r.Out() {
			switch v := p.(type) {
			case agent.TracePayload:
				if v.HostName != "asdq" {
					t.Fatalf("bad hostname, wanted %q, got %q", "asdq", v.HostName)
				}
			case agent.StatsPayload:
				continue
			}
			break
		}
	})

	t.Run("no-config", func(t *testing.T) {
		r.T = t
		if err := r.RunAgent([]byte(``)); err != nil {
			t.Fatal(err)
		}

		traceList := agent.Traces{
			agent.Trace{test.NewSpan(nil)},
			agent.Trace{test.NewSpan(nil)},
		}
		for i := 0; i < 10; i++ {
			go func() {
				if err := r.Post(traceList); err != nil {
					t.Fatal(err)
				}
			}()
		}

		for p := range r.Out() {
			switch v := p.(type) {
			case agent.TracePayload:
				t.Logf("OK traces (host:%q, count:%d)\n", v.HostName, len(v.Traces))
			case agent.StatsPayload:
				continue
			}
			break
		}
	})

	if err := r.Shutdown(time.Second); err != nil {
		t.Fatal(err)
	}
}

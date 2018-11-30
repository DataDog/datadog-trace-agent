package main

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/DataDog/datadog-trace-agent/internal/testutil"
	"github.com/DataDog/datadog-trace-agent/test"
)

func TestHostname(t *testing.T) {
	r := test.Runner{}
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Shutdown(time.Second); err != nil {
			t.Log("shutdown: trace-agent might still be running: ", err)
		}
	}()

	t.Run("from-config", func(t *testing.T) {
		if err := r.RunAgent([]byte(`hostname: asdq`)); err != nil {
			t.Fatal(err)
		}
		defer r.StopAgent()

		traceList := agent.Traces{
			agent.Trace{testutil.RandomSpan()},
			agent.Trace{testutil.RandomSpan()},
			agent.Trace{testutil.RandomSpan()},
			agent.Trace{testutil.RandomSpan()},
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
				if n := len(v.Traces); n != 40 {
					t.Fatalf("expected %d traces, got %d", len(traceList), n)
				}
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
		if err := r.RunAgent([]byte(``)); err != nil {
			t.Fatal(err)
		}
		defer r.StopAgent()

		traceList := agent.Traces{
			agent.Trace{testutil.RandomSpan()},
			agent.Trace{testutil.RandomSpan()},
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
				if n := len(v.Traces); n != 20 {
					t.Fatalf("expected %d traces, got %d", len(traceList), n)
				}
				if v.HostName == "" {
					t.Fatal("got empty hostname")
				}
			case agent.StatsPayload:
				continue
			}
			break
		}
	})
}

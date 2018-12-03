package testsuite

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/DataDog/datadog-trace-agent/internal/test"
	"github.com/DataDog/datadog-trace-agent/internal/test/testutil"
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
		defer r.KillAgent()

		payload := agent.Traces{agent.Trace{testutil.RandomSpan()}}
		if err := r.Post(payload); err != nil {
			t.Fatal(err)
		}
		waitForTrace(t, r.Out(), func(v agent.TracePayload) {
			if n := len(v.Traces); n != 1 {
				t.Fatalf("expected %d traces, got %d", len(payload), n)
			}
			if v.HostName == "" {
				t.Fatal("got empty hostname")
			}
		})
	})

	t.Run("no-config", func(t *testing.T) {
		if err := r.RunAgent([]byte(``)); err != nil {
			t.Fatal(err)
		}
		defer r.KillAgent()

		payload := agent.Traces{agent.Trace{testutil.RandomSpan()}}
		if err := r.Post(payload); err != nil {
			t.Fatal(err)
		}
		waitForTrace(t, r.Out(), func(v agent.TracePayload) {
			if n := len(v.Traces); n != 1 {
				t.Fatalf("expected %d traces, got %d", len(payload), n)
			}
			if v.HostName == "" {
				t.Fatal("got empty hostname")
			}
		})
	})
}

// waitForTrace waits on the out channel until it times out or receives an agent.TracePayload.
// If the latter happens it will call fn.
func waitForTrace(t *testing.T, out <-chan interface{}, fn func(agent.TracePayload)) {
	waitForTraceTimeout(t, out, 3*time.Second, fn)
}

// waitForTraceTimeout behaves like waitForTrace but allows a customizable wait time.
func waitForTraceTimeout(t *testing.T, out <-chan interface{}, wait time.Duration, fn func(agent.TracePayload)) {
	timeout := time.After(wait)
	for {
		select {
		case p := <-out:
			if v, ok := p.(agent.TracePayload); ok {
				fn(v)
				return
			}
		case <-timeout:
			t.Fatal("timed out")
		}
	}
}

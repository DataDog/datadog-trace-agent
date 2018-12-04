package testsuite

import (
	"os"
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
			t.Log("shutdown: ", err)
		}
	}()

	// testHostname returns a test which asserts that for the given agent conf, the
	// expectedHostname is sent to the backend.
	testHostname := func(conf []byte, expectedHostname string) func(*testing.T) {
		return func(t *testing.T) {
			if err := r.RunAgent(conf); err != nil {
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
				if v.HostName != expectedHostname {
					t.Fatalf("expected %q, got %q", expectedHostname, v.HostName)
				}
			})
		}
	}

	t.Run("from-config", testHostname([]byte(`hostname: asdq`), "asdq"))

	t.Run("env", func(t *testing.T) {
		os.Setenv("DD_HOSTNAME", "my-env-host")
		defer os.Unsetenv("DD_HOSTNAME")
		testHostname([]byte(`hostname: my-host`), "my-env-host")(t)
	})

	t.Run("auto", func(t *testing.T) {
		if err := r.RunAgent(nil); err != nil {
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
				t.Fatal("hostname detection failed")
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

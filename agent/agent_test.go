package main

import (
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
)

func TestWatchdog(t *testing.T) {
	if testing.Short() {
		return
	}

	conf := config.NewDefaultAgentConfig()
	conf.APIKeys = append(conf.APIKeys, "apikey_2")
	conf.MaxMemory = 1e7
	conf.WatchdogInterval = time.Millisecond

	// save the global mux aside, we don't want to break other tests
	defaultMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	agent := NewAgent(conf)

	defer func() {
		close(agent.exit)
		// We need to manually close the receiver as the Run() func
		// should have been broken and interrupted by the watchdog panic
		close(agent.Receiver.exit)
		// we need to wait more than on second (time for StoppableListener.Accept
		// to acknowledge the connection has been closed)
		time.Sleep(2 * time.Second)
		http.DefaultServeMux = defaultMux
	}()

	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case string:
				if strings.HasPrefix(v, "exceeded max memory") {
					t.Logf("watchdog worked, trapped the right error: %s", v)
					runtime.GC() // make sure we clean up after allocating all this
					return
				}
			}
			t.Fatalf("unexpected error: %v", r)
		}
	}()

	// allocating a lot of memory
	buf := make([]byte, 2*int64(conf.MaxMemory))
	buf[0] = 1
	buf[len(buf)-1] = 1

	// after some time, the watchdog should kill this
	agent.Run()

	// without this. runtime could be smart and free memory before we Run()
	buf[0] = 2
	buf[len(buf)-1] = 2
}

func BenchmarkAgentTraceProcessing(b *testing.B) {
	// Disable debug logs in these tests
	config.NewLoggerLevelCustom("INFO", "/var/log/datadog/trace-agent.log")

	conf := config.NewDefaultAgentConfig()
	conf.APIKeys = append(conf.APIKeys, "")
	agent := NewAgent(conf)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agent.Process(fixtures.RandomTrace(10, 8))
	}
}

func BenchmarkWatchdog(b *testing.B) {
	conf := config.NewDefaultAgentConfig()
	conf.APIKeys = append(conf.APIKeys, "apikey_2")
	agent := NewAgent(conf)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agent.watchdog()
	}
}

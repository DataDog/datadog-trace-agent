package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/quantizer"
	"github.com/stretchr/testify/assert"
)

func TestWatchdog(t *testing.T) {
	if testing.Short() {
		return
	}

	conf := config.NewDefaultAgentConfig()
	conf.APIKey = "apikey_2"
	conf.MaxMemory = 1e7
	conf.WatchdogInterval = time.Millisecond

	// save the global mux aside, we don't want to break other tests
	defaultMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	exit := make(chan struct{})
	agent := NewAgent(conf, exit)

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

	// override the default die, else our test would stop, use a plain panic() instead
	agent.die = func(format string, args ...interface{}) {
		panic(fmt.Sprintf(format, args...))
	}

	// after some time, the watchdog should kill this
	agent.Run()

	// without this. runtime could be smart and free memory before we Run()
	buf[0] = 2
	buf[len(buf)-1] = 2
}

// Test to make sure that the joined effort of the quantizer and truncator, in that order, produce the
// desired string
func TestFormatTrace(t *testing.T) {
	assert := assert.New(t)
	resource := "SELECT name FROM people WHERE age = 42"
	rep := strings.Repeat(" AND age = 42", 5000)
	resource = resource + rep
	testTrace := model.Trace{
		&model.Span{
			Resource: resource,
			Type:     "sql",
		},
	}
	result := formatTrace(testTrace)[0]

	assert.Equal(5000, len(result.Resource))
	assert.NotEqual("Non-parsable SQL query", result.Resource)
	assert.NotContains(result.Resource, "42")
	assert.Contains(result.Resource, "SELECT name FROM people WHERE age = ?")

	assert.Equal(5003, len(result.Meta["sql.query"])) // Ellipsis added in quantizer
	assert.NotEqual("Non-parsable SQL query", result.Meta["sql.query"])
	assert.NotContains(result.Meta["sql.query"], "42")
	assert.Contains(result.Meta["sql.query"], "SELECT name FROM people WHERE age = ?")
}

func BenchmarkAgentTraceProcessing(b *testing.B) {
	c := config.NewDefaultAgentConfig()
	c.APIKey = "test"

	runTraceProcessingBenchmark(b, c)
}

func BenchmarkAgentTraceProcessingWithFiltering(b *testing.B) {
	c := config.NewDefaultAgentConfig()
	c.APIKey = "test"
	c.Ignore["resource"] = []string{"[0-9]{3}", "foobar", "G.T [a-z]+", "[^123]+_baz"}

	runTraceProcessingBenchmark(b, c)
}

// worst case scenario: spans are tested against multiple rules without any match.
// this means we won't compesate the overhead of filtering by dropping traces
func BenchmarkAgentTraceProcessingWithWorstCaseFiltering(b *testing.B) {
	c := config.NewDefaultAgentConfig()
	c.APIKey = "test"
	c.Ignore["resource"] = []string{"[0-9]{3}", "foobar", "aaaaa?aaaa", "[^123]+_baz"}

	runTraceProcessingBenchmark(b, c)
}

func runTraceProcessingBenchmark(b *testing.B, c *config.AgentConfig) {
	exit := make(chan struct{})
	agent := NewAgent(c, exit)
	log.UseLogger(log.Disabled)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agent.Process(fixtures.RandomTrace(10, 8))
	}
}

func BenchmarkWatchdog(b *testing.B) {
	conf := config.NewDefaultAgentConfig()
	conf.APIKey = "apikey_2"
	exit := make(chan struct{})
	agent := NewAgent(conf, exit)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agent.watchdog()
	}
}

// Mimicks behaviour of agent Process function
func formatTrace(t model.Trace) model.Trace {
	for i := range t {
		quantizer.Quantize(t[i])
		t[i].Truncate()
	}
	return t
}

func TestAgentWithTransactions(t *testing.T) {
	fullTraceTest(t, 100, true)
}

func TestAgentWithoutTransactions(t *testing.T) {
	fullTraceTest(t, 100, false)
}

func fullTraceTest(t *testing.T, numTraces int, transactions bool) {
	if testing.Short() {
		t.Skip("Skipping full trace short since we're running only short tests")
	}

	// Disable logs
	log.UseLogger(log.Disabled)
	defer log.UseLogger(log.Default)

	// Create a listener on a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	numSpansPerTrace := 20

	exit := make(chan struct{})

	// Create agent config
	c := config.NewDefaultAgentConfig()
	// Make agent send to listener we created at the beginning
	c.APIEndpoint = fmt.Sprintf("http://%s", listener.Addr().String())
	c.ReceiverPort = findFreePort()
	c.APIKey = "apikey_2"
	// Force each trace to go on its own payload to simplify end condition
	c.TraceWriterConfig.FlushPeriod = 2 * time.Hour
	c.TraceWriterConfig.MaxSpansPerPayload = numSpansPerTrace

	// Set service to extract transactions from
	transactionService := "mysql"
	if transactions {
		c.AnalyzedRateByService = map[string]float64{
			transactionService: 1,
		}
	}

	// Create agent
	agent := NewAgent(c, exit)

	rand.Seed(1)

	// Create test traces
	traces := make([]model.Trace, 0, numTraces)
	for i := 0; i < numTraces; i++ {
		trace := fixtures.RandomFixedSizeTrace(numSpansPerTrace)

		// Panic if the number generator failed to preserve number of spans
		if len(trace) != numSpansPerTrace {
			panic(trace)
		}

		root := trace.GetRoot()

		// Make sure all traces survive sampling
		root.Metrics[samplingPriorityKey] = 2

		// If we care about transactions lets set all root spans to the service transactions are being extracted from.
		if transactions {
			root.Service = transactionService
		}

		traces = append(traces, trace)
	}

	// Temporarily overwrite global servemux to prevent multiple func registration errors
	oldDefaultServeMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
	defer func() {
		http.DefaultServeMux = oldDefaultServeMux
	}()

	// Start the agent with a waitgroup signaling when it stops
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		agent.Run()
		wg.Done()
	}()

	// Create a mock http server tracking number of trace requests received and total trace request bytes.
	totalBytesReceived := int64(0)
	// Keep track of received trace payloads in a waitgroup so we can wait for this at the end
	reqWg := sync.WaitGroup{}
	reqWg.Add(len(traces))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v0.2/traces", func(w http.ResponseWriter, r *http.Request) {
		if transactions {
			// When dealing with transactions, it's possible that there's one extra payload at the end containing a
			// single transaction so reqWg.Done() might be called one more time than expected.
			defer func() {
				recover()
			}()
		}
		atomic.AddInt64(&totalBytesReceived, r.ContentLength)
		w.WriteHeader(200)
		reqWg.Done()
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// Start said server
	srv := http.Server{Addr: listener.Addr().String(), Handler: mux}
	go srv.Serve(listener)

	start := time.Now()

	// Send the test traces to the agent receiver
	for _, trace := range traces {
		agent.Receiver.traces <- trace
	}

	// Wait for our mock server to acknowledge all trace payloads
	reqWg.Wait()

	fmt.Printf("Took %v\n", time.Since(start))
	fmt.Printf("Wrote %fMB of data\n", float64(totalBytesReceived)/1024/1024)

	// Stop the agent
	close(exit)
	wg.Wait()
	// And the server
	srv.Close()
}

// findFreePort returns a free tcp port or panics
func findFreePort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

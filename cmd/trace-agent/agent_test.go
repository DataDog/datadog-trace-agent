package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/obfuscate"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/testutil"
	"github.com/stretchr/testify/assert"
)

func TestWatchdog(t *testing.T) {
	if testing.Short() {
		return
	}

	conf := config.New()
	conf.Endpoints[0].APIKey = "apikey_2"
	conf.MaxMemory = 1e7
	conf.WatchdogInterval = time.Millisecond

	// save the global mux aside, we don't want to break other tests
	defaultMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	ctx, cancelFunc := context.WithCancel(context.Background())
	agent := NewAgent(ctx, conf)

	defer func() {
		cancelFunc()
		// We need to manually close the receiver as the Run() func
		// should have been broken and interrupted by the watchdog panic
		agent.Receiver.Stop()
		http.DefaultServeMux = defaultMux
	}()

	var killed bool
	defer func() {
		if r := recover(); r != nil {
			killed = true
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
	oldDie := dieFunc
	defer func() { dieFunc = oldDie }()
	dieFunc = func(format string, args ...interface{}) {
		panic(fmt.Sprintf(format, args...))
	}

	// after some time, the watchdog should kill this
	agent.Run()

	// without this. runtime could be smart and free memory before we Run()
	buf[0] = 2
	buf[len(buf)-1] = 2

	assert.True(t, killed)
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

func TestProcess(t *testing.T) {
	t.Run("Replacer", func(t *testing.T) {
		// Ensures that for "sql" type spans:
		// • obfuscator runs before replacer
		// • obfuscator obfuscates both resource and "sql.query" tag
		// • resulting resource is obfuscated with replacements applied
		// • resulting "sql.query" tag is obfuscated with no replacements applied
		cfg := config.New()
		cfg.Endpoints[0].APIKey = "test"
		cfg.ReplaceTags = []*config.ReplaceRule{{
			Name: "resource.name",
			Re:   regexp.MustCompile("AND.*"),
			Repl: "...",
		}}
		ctx, cancel := context.WithCancel(context.Background())
		agent := NewAgent(ctx, cfg)
		defer cancel()

		now := time.Now()
		span := &model.Span{
			Resource: "SELECT name FROM people WHERE age = 42 AND extra = 55",
			Type:     "sql",
			Start:    now.Add(-time.Second).UnixNano(),
			Duration: (500 * time.Millisecond).Nanoseconds(),
		}
		agent.Process(model.Trace{span})

		assert := assert.New(t)
		assert.Equal("SELECT name FROM people WHERE age = ? ...", span.Resource)
		assert.Equal("SELECT name FROM people WHERE age = ? AND extra = ?", span.Meta["sql.query"])
	})

	t.Run("Blacklister", func(t *testing.T) {
		cfg := config.New()
		cfg.Endpoints[0].APIKey = "test"
		cfg.Ignore["resource"] = []string{"^INSERT.*"}
		ctx, cancel := context.WithCancel(context.Background())
		agent := NewAgent(ctx, cfg)
		defer cancel()

		now := time.Now()
		spanValid := &model.Span{
			Resource: "SELECT name FROM people WHERE age = 42 AND extra = 55",
			Type:     "sql",
			Start:    now.Add(-time.Second).UnixNano(),
			Duration: (500 * time.Millisecond).Nanoseconds(),
		}
		spanInvalid := &model.Span{
			Resource: "INSERT INTO db VALUES (1, 2, 3)",
			Type:     "sql",
			Start:    now.Add(-time.Second).UnixNano(),
			Duration: (500 * time.Millisecond).Nanoseconds(),
		}

		stats := agent.Receiver.Stats.GetTagStats(info.Tags{})
		assert := assert.New(t)

		agent.Process(model.Trace{spanValid})
		assert.EqualValues(0, stats.TracesFiltered)
		assert.EqualValues(0, stats.SpansFiltered)

		agent.Process(model.Trace{spanInvalid, spanInvalid})
		assert.EqualValues(1, stats.TracesFiltered)
		assert.EqualValues(2, stats.SpansFiltered)
	})

	t.Run("Stats/Priority", func(t *testing.T) {
		cfg := config.New()
		cfg.Endpoints[0].APIKey = "test"
		ctx, cancel := context.WithCancel(context.Background())
		agent := NewAgent(ctx, cfg)
		defer cancel()

		now := time.Now()
		disabled := float64(-99)
		for _, key := range []float64{
			disabled, -1, -1, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 2,
		} {
			span := &model.Span{
				Resource: "SELECT name FROM people WHERE age = 42 AND extra = 55",
				Type:     "sql",
				Start:    now.Add(-time.Second).UnixNano(),
				Duration: (500 * time.Millisecond).Nanoseconds(),
				Metrics:  map[string]float64{},
			}
			if key != disabled {
				span.Metrics[sampler.SamplingPriorityKey] = key
			}
			agent.Process(model.Trace{span})
		}

		stats := agent.Receiver.Stats.GetTagStats(info.Tags{})
		assert.EqualValues(t, 1, stats.TracesPriorityNone)
		assert.EqualValues(t, 2, stats.TracesPriorityNeg)
		assert.EqualValues(t, 3, stats.TracesPriority0)
		assert.EqualValues(t, 4, stats.TracesPriority1)
		assert.EqualValues(t, 5, stats.TracesPriority2)
	})
}

func TestSampling(t *testing.T) {
	cfg := config.New()
	cfg.APIKey = "test"
	ctx, cancel := context.WithCancel(context.Background())
	agent := NewAgent(ctx, cfg)
	defer cancel()

	disabled := float64(-99)
	for i := 0; i < 200; i++ {
		for j, key := range []float64{disabled, -1, 0, 0, 1, 1, 2} {

			span := &model.Span{
				Service:  "serv1",
				Start:    time.Now().UnixNano(),
				Duration: (100 * time.Millisecond).Nanoseconds(), Metrics: map[string]float64{}}
			trace := model.Trace{span}
			pt := processedTrace{
				Trace: trace,
				Root:  span}
			clientPriorityRate := 0.2
			hasPriority := false
			if key != disabled {
				span.Metrics[sampler.SamplingPriorityKey] = key
				hasPriority = true
				if j%2 == 0 {
					span.Metrics[sampler.SamplingPriorityRateKey] = clientPriorityRate
				}
			}
			// totalTraces := i*j + j + 1
			agent.sample(pt, hasPriority)
			sampleRate := sampler.GetTraceAppliedSampleRate(span)
			if key == 2 {
				assert.Equal(t, 1.0, sampleRate, "always sample when priority above 1")
				continue
			}

			scoreSignature := sampler.ComputeSignatureWithRootAndEnv(trace, span, "")
			scoreSampler := agent.ScoreSampler.engine.(*sampler.ScoreEngine)
			scoreRate := scoreSampler.Sampler.GetSampleRate(trace, span, scoreSignature)

			if key == disabled || key == -1 {
				// The applied rate is the score sampler rate
				assert.Equal(t, scoreRate, sampleRate, "only scoresampler is applied when priority is under 0")
				continue
			}

			if j%2 == 0 {
				expectedRate := sampler.MergeParallelSamplingRates(clientPriorityRate, scoreRate)
				assert.Equal(t, expectedRate, sampleRate, "score sampler rate in parallel priority rate applied by agent")
				continue
			}
			serviceSignature := sampler.ComputeServiceSignature(root, env)
			prioritySampler := agent.PrioritySampler.engine.(*sampler.PriorityEngine)
			priorityRate := prioritySampler.Sampler.GetSignatureSampleRate(serviceSignature)
			expectedRate := sampler.MergeParallelSamplingRates(priorityRate, scoreRate)
			assert.Equal(t, expectedRate, sampleRate, "score sampler rate in parallel to next priority rate")
		}
	}
}

func BenchmarkAgentTraceProcessing(b *testing.B) {
	c := config.New()
	c.Endpoints[0].APIKey = "test"

	runTraceProcessingBenchmark(b, c)
}

func BenchmarkAgentTraceProcessingWithFiltering(b *testing.B) {
	c := config.New()
	c.Endpoints[0].APIKey = "test"
	c.Ignore["resource"] = []string{"[0-9]{3}", "foobar", "G.T [a-z]+", "[^123]+_baz"}

	runTraceProcessingBenchmark(b, c)
}

// worst case scenario: spans are tested against multiple rules without any match.
// this means we won't compesate the overhead of filtering by dropping traces
func BenchmarkAgentTraceProcessingWithWorstCaseFiltering(b *testing.B) {
	c := config.New()
	c.Endpoints[0].APIKey = "test"
	c.Ignore["resource"] = []string{"[0-9]{3}", "foobar", "aaaaa?aaaa", "[^123]+_baz"}

	runTraceProcessingBenchmark(b, c)
}

func runTraceProcessingBenchmark(b *testing.B, c *config.AgentConfig) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	agent := NewAgent(ctx, c)
	log.UseLogger(log.Disabled)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agent.Process(testutil.RandomTrace(10, 8))
	}
}

func BenchmarkWatchdog(b *testing.B) {
	conf := config.New()
	conf.Endpoints[0].APIKey = "apikey_2"
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	agent := NewAgent(ctx, conf)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agent.watchdog()
	}
}

// Mimicks behaviour of agent Process function
func formatTrace(t model.Trace) model.Trace {
	for _, span := range t {
		obfuscate.NewObfuscator(nil).Obfuscate(span)
		span.Truncate()
	}
	return t
}

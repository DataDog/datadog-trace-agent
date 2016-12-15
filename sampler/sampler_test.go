package sampler

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func getTestSampler() *Sampler {
	// Disable debug logs in these tests
	config.NewLoggerLevelCustom("INFO", "/var/log/datadog/trace-agent.log")

	// No extra fixed sampling, no maximum TPS
	extraRate := 1.0
	maxTPS := 0.0

	return NewSampler(extraRate, maxTPS)
}

func getTestTrace() (model.Trace, *model.Span) {
	tID := randomTraceID()
	trace := model.Trace{
		model.Span{TraceID: tID, SpanID: 1, ParentID: 0, Start: 42, Duration: 1000000, Service: "mcnulty", Type: "web"},
		model.Span{TraceID: tID, SpanID: 2, ParentID: 1, Start: 100, Duration: 200000, Service: "mcnulty", Type: "sql"},
	}
	return trace, &trace[0]
}

func TestSamplerLoop(t *testing.T) {
	s := getTestSampler()

	exit := make(chan bool)

	go func() {
		s.Run()
		close(exit)
	}()

	s.Stop()

	select {
	case <-exit:
		return
	case <-time.After(time.Second * 1):
		assert.Fail(t, "Sampler took more than 1 second to close")
	}
}

func TestExtraSampleRate(t *testing.T) {
	assert := assert.New(t)

	s := getTestSampler()
	trace, root := getTestTrace()
	signature := ComputeSignature(trace)

	// Feed the s with a signature so that it has a < 1 sample rate
	for i := 0; i < int(1e6); i++ {
		s.Sample(trace)
	}

	sRate := s.GetSampleRate(trace, root, signature)

	// Then turn on the extra sample rate, then ensure it affects both existing and new signatures
	s.extraRate = 0.33

	assert.Equal(s.GetSampleRate(trace, root, signature), s.extraRate*sRate)
}

func TestMaxTPS(t *testing.T) {
	// Test the "effectiveness" of the maxTPS option.
	assert := assert.New(t)
	s := getTestSampler()

	maxTPS := 5.0
	tps := 100.0
	// To avoid the edge effects from an non-initialized sampler, wait a bit before counting samples.
	initPeriods := 20
	periods := 50

	s.maxTPS = maxTPS
	periodSeconds := s.Backend.decayPeriod.Seconds()
	tracesPerPeriod := tps * periodSeconds
	// Set signature score offset high enough not to kick in during the test.
	s.signatureScoreOffset = 2 * tps
	s.signatureScoreCoefficient = math.Pow(s.signatureScoreSlope, math.Log10(s.signatureScoreOffset))

	sampledCount := 0

	for period := 0; period < initPeriods+periods; period++ {
		s.Backend.DecayScore()
		for i := 0; i < int(tracesPerPeriod); i++ {
			trace, _ := getTestTrace()
			sampled := s.Sample(trace)
			// Once we got into the "supposed-to-be" stable "regime", count the samples
			if period > initPeriods && sampled {
				sampledCount++
			}
		}
	}

	// Check that the sampled score pre-maxTPS is equals to the incoming number of traces per second
	assert.InEpsilon(tps, s.Backend.GetSampledScore(), 0.01)

	// We should have kept less traces per second than maxTPS
	assert.True(s.maxTPS >= float64(sampledCount)/(float64(periods)*periodSeconds))

	// We should have a throughput of sampled traces around maxTPS
	// Check for 1% epsilon, but the precision also depends on the backend imprecision (error factor = decayFactor).
	// Combine error rates with L1-norm instead of L2-norm by laziness, still good enough for tests.
	assert.InEpsilon(s.maxTPS, float64(sampledCount)/(float64(periods)*periodSeconds),
		0.01+s.Backend.decayFactor-1)
}

func TestSamplerChainedSampling(t *testing.T) {
	assert := assert.New(t)
	s := getTestSampler()

	trace, _ := getTestTrace()

	root := trace.GetRoot()

	// Received trace already got sampled
	SetTraceAppliedSampleRate(root, 0.8)
	assert.Equal(0.8, GetTraceAppliedSampleRate(root))

	// Sample again with an ensured rate, rates should be combined
	s.extraRate = 0.5
	s.Sample(trace)
	assert.Equal(0.4, GetTraceAppliedSampleRate(root))

	// Check the sample rate isn't lost by reference
	rootAgain := trace.GetRoot()
	assert.Equal(0.4, GetTraceAppliedSampleRate(rootAgain))
}

func BenchmarkSampler(b *testing.B) {
	// Benchmark the resource consumption of many traces sampling

	// Up to signatureCount different signatures
	signatureCount := 20

	s := getTestSampler()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		trace := model.Trace{
			model.Span{TraceID: 1, SpanID: 1, ParentID: 0, Start: 42, Duration: 1000000000, Service: "mcnulty", Type: "web", Resource: string(rand.Intn(signatureCount))},
			model.Span{TraceID: 1, SpanID: 2, ParentID: 1, Start: 100, Duration: 200000000, Service: "mcnulty", Type: "sql"},
			model.Span{TraceID: 1, SpanID: 3, ParentID: 2, Start: 150, Duration: 199999000, Service: "master-db", Type: "sql"},
			model.Span{TraceID: 1, SpanID: 4, ParentID: 1, Start: 500000000, Duration: 500000, Service: "redis", Type: "redis"},
			model.Span{TraceID: 1, SpanID: 5, ParentID: 1, Start: 700000000, Duration: 700000, Service: "mcnulty", Type: ""},
		}
		s.Sample(trace)
	}
}

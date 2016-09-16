package sampler

import (
	"math/rand"
	"testing"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func getTestSampler() *Sampler {
	// Disable debug logs in these tests
	config.NewLoggerLevel(false)

	extraRate := 1.0

	return NewSampler(extraRate)
}

func getTestTrace() (model.Trace, *model.Span) {
	trace := model.Trace{
		model.Span{TraceID: 777, SpanID: 1, ParentID: 0, Start: 42, Duration: 1000000, Service: "mcnulty", Type: "web"},
		model.Span{TraceID: 777, SpanID: 2, ParentID: 1, Start: 100, Duration: 200000, Service: "mcnulty", Type: "sql"},
	}
	return trace, &trace[0]
}

func TestSamplerLoop(t *testing.T) {
	sampler := getTestSampler()

	exit := make(chan bool)

	go func() {
		sampler.Run()
		close(exit)
	}()

	sampler.Stop()

	for {
		select {
		case <-exit:
			return
		case <-time.After(time.Second * 1):
			assert.Fail(t, "Sampler took more than 1 second to close")
		}
	}
}

func TestExtraSampleRate(t *testing.T) {
	assert := assert.New(t)

	sampler := getTestSampler()
	trace, root := getTestTrace()
	signature := ComputeSignature(trace)

	// Feed the sampler with a signature so that it has a < 1 sample rate
	for i := 0; i < int(1e6); i++ {
		sampler.Sample(trace)
	}

	sRate := sampler.GetSampleRate(trace, root, signature)

	// Then turn on the extra sample rate, then ensure it affects both existing and new signatures
	sampler.extraRate = 0.33

	assert.Equal(sampler.GetSampleRate(trace, root, signature), sampler.extraRate*sRate)
}

func TestSamplerChainedSampling(t *testing.T) {
	assert := assert.New(t)
	sampler := getTestSampler()

	trace, _ := getTestTrace()

	root := GetRoot(trace)

	// Received trace already got sampled
	SetTraceSampleRate(root, 0.8)
	assert.Equal(0.8, GetTraceSampleRate(root))

	// Sample again with an ensured rate, rates should be combined
	sampler.extraRate = 0.5
	sampler.Sample(trace)
	assert.Equal(0.4, GetTraceSampleRate(root))

	// Check the sample rate isn't lost by reference
	rootAgain := GetRoot(trace)
	assert.Equal(0.4, GetTraceSampleRate(rootAgain))
}

func BenchmarkSampler(b *testing.B) {
	// Benchmark the resource consumption of many traces sampling

	// Up to signatureCount different signatures
	signatureCount := 20

	sampler := getTestSampler()

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
		sampler.Sample(trace)
	}
}

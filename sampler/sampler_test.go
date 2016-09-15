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

	sampler := NewSampler()

	return sampler
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

func BenchmarkSampler(b *testing.B) {
	// Benchmark the resource consumption of many traces sampling

	// Up to signatureCount different signatures
	signatureCount := 20

	sampler := getTestSampler()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tr := model.Trace{
			model.Span{TraceID: 1, SpanID: 1, ParentID: 0, Start: 42, Duration: 1000000000, Service: "mcnulty", Type: "web", Resource: string(rand.Intn(signatureCount))},
			model.Span{TraceID: 1, SpanID: 2, ParentID: 1, Start: 100, Duration: 200000000, Service: "mcnulty", Type: "sql"},
			model.Span{TraceID: 1, SpanID: 3, ParentID: 2, Start: 150, Duration: 199999000, Service: "master-db", Type: "sql"},
			model.Span{TraceID: 1, SpanID: 4, ParentID: 1, Start: 500000000, Duration: 500000, Service: "redis", Type: "redis"},
			model.Span{TraceID: 1, SpanID: 5, ParentID: 1, Start: 700000000, Duration: 700000, Service: "mcnulty", Type: ""},
		}
		sampler.IsSample(tr)
	}
}

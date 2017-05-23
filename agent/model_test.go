// Some benchmarks defined here because it both requires fixtures & model
// and putting them in model would cause a circular dependency.

package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	defaultEnv = "dev"
)

func BenchmarkHandleSpanRandom(b *testing.B) {
	sb := model.NewStatsRawBucket(0, 1e9)
	aggr := []string{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trace := fixtures.RandomTrace(10, 8)
		root := trace.GetRoot()
		trace.ComputeWeight(*root)
		trace.ComputeTopLevel()
		for _, span := range trace {
			sb.HandleSpan(span, defaultEnv, aggr, nil)
		}
	}
}

package quantile

import (
	"math/rand"
	"testing"
)

const randlen = 1000

func randSlice() []float64 {
	// use those
	vals := make([]float64, 0, randlen)
	for i := 0; i < randlen; i++ {
		vals = append(vals, rand.Float64())
	}

	return vals
}

func BenchmarkGKSkiplistRandom(b *testing.B) {
	s := NewSummary()

	vals := randSlice()
	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		s.Insert(vals[n%randlen], uint64(n))
	}
}

func BenchmarkGKDLLRandom(b *testing.B) {
	s := NewSimpleSummary()

	vals := randSlice()

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		s.Insert(vals[n%randlen], uint64(n))
	}
}

func BenchmarkGKSliceRandom(b *testing.B) {
	s := NewSliceSummary()

	vals := randSlice()

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		s.Insert(vals[n%randlen], uint64(n))
	}
}

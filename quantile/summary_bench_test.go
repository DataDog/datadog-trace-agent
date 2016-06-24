package quantile

import (
	"encoding/json"
	"math/rand"
	"testing"
)

const randlen = 1000

func randSlice(n int) []float64 {
	// use those
	vals := make([]float64, 0, randlen)
	for i := 0; i < n; i++ {
		vals = append(vals, rand.Float64()*100000)
	}

	return vals
}

func BenchmarkGKInsertion(b *testing.B) {
	s := NewSummary()

	vals := randSlice(randlen)

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		s.Insert(vals[n%randlen], uint64(n))
	}
}

func BenchmarkGKInsertionPreallocd(b *testing.B) {
	s := NewSummary()
	s.Entries = make([]Entry, 0, 100)

	vals := randSlice(randlen)

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		s.Insert(vals[n%randlen], uint64(n))
	}
}

func BGKQuantiles(b *testing.B, n int) {
	s := NewSummary()
	vals := randSlice(n)
	for i := 0; i < n; i++ {
		s.Insert(vals[i], uint64(i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		s.Quantile(rand.Float64())
	}
}
func BenchmarkGKQuantiles10(b *testing.B) {
	BGKQuantiles(b, 10)
}
func BenchmarkGKQuantiles100(b *testing.B) {
	BGKQuantiles(b, 100)
}
func BenchmarkGKQuantiles1000(b *testing.B) {
	BGKQuantiles(b, 1000)
}
func BenchmarkGKQuantiles10000(b *testing.B) {
	BGKQuantiles(b, 10000)
}
func BenchmarkGKQuantiles100000(b *testing.B) {
	BGKQuantiles(b, 100000)
}

// Encoding tests
func BGKEncoding(b *testing.B, n int) {
	s := NewSummary()
	vals := randSlice(n)
	for i := 0; i < n; i++ {
		s.Insert(vals[i], uint64(i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		blob, _ := json.Marshal(&s)
		var ss Summary
		json.Unmarshal(blob, &ss)
	}
}
func BenchmarkGKEncoding10(b *testing.B) {
	BGKEncoding(b, 10)
}
func BenchmarkGKEncoding100(b *testing.B) {
	BGKEncoding(b, 100)
}

// not worth encoding larger as we're constant in mem
func BenchmarkGKEncoding1000(b *testing.B) {
	BGKEncoding(b, 1000)
}

package model

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

func benchmarkFloatSlice(n int, b *testing.B) {
	// Initialize a proper slice
	data := make([]float64, n)
	for j := 0; j < n; j++ {
		data = append(data, rand.Float64()*100)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sort.Float64s(data)
	}
}

func BenchmarkFloatSlice10(b *testing.B)      { benchmarkFloatSlice(10, b) }
func BenchmarkFloatSlice100(b *testing.B)     { benchmarkFloatSlice(100, b) }
func BenchmarkFloatSlice1000(b *testing.B)    { benchmarkFloatSlice(1000, b) }
func BenchmarkFloatSlice1000000(b *testing.B) { benchmarkFloatSlice(1000000, b) }

func benchmarkFloatBitsSlice(n int, b *testing.B) {
	// Initialize a proper slice
	data := make(FloatBitsSlice, n)
	for j := 0; j < n; j++ {
		data = append(data, math.Float64bits(rand.Float64()*100))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sort.Sort(data)
	}
}

func BenchmarkFloatBitsSlice10(b *testing.B)      { benchmarkFloatBitsSlice(10, b) }
func BenchmarkFloatBitsSlice100(b *testing.B)     { benchmarkFloatBitsSlice(100, b) }
func BenchmarkFloatBitsSlice1000(b *testing.B)    { benchmarkFloatBitsSlice(1000, b) }
func BenchmarkFloatBitsSlice1000000(b *testing.B) { benchmarkFloatBitsSlice(1000000, b) }

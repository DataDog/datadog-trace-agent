package quantile

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

/************************************************************************************
	DATA VALIDATION, with different strategies make sure of the correctness of
	our epsilon-approximate quantiles
************************************************************************************/

var testQuantiles = []float64{0, 0.1, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 0.999, 0.9999, 1}

func GenSummary(n int, gen func(i int) float64) ([]float64, []uint64) {
	s := NewSummary()

	for i := 0; i < n; i++ {
		s.Insert(gen(i), uint64(i))
	}

	vals := make([]float64, 0, len(testQuantiles))
	samps := make([]uint64, 0, len(testQuantiles))
	for _, q := range testQuantiles {
		val, samp := s.Quantile(q)
		vals = append(vals, val)
		samps = append(samps, samp...)
	}

	return vals, samps
}

/* CONSTANT STREAMS
   The most simple checker
*/
func ConstantGenerator(i int) float64 {
	return 42
}
func SummaryConstantN(t *testing.T, n int) {
	assert := assert.New(t)
	vals, _ := GenSummary(n, ConstantGenerator)
	for _, v := range vals {
		assert.Equal(42, v)
	}
}
func TestSummaryConstant10(t *testing.T) {
	SummaryConstantN(t, 10)
}
func TestSummaryConstant100(t *testing.T) {
	SummaryConstantN(t, 100)
}
func TestSummaryConstant1000(t *testing.T) {
	SummaryConstantN(t, 1000)
}
func TestSummaryConstant10000(t *testing.T) {
	SummaryConstantN(t, 10000)
}
func TestSummaryConstant100000(t *testing.T) {
	SummaryConstantN(t, 100000)
}

/* uniform distribution
   expected quantiles are easily to compute as the value == its rank
   1 to i
*/
func UniformGenerator(i int) float64 {
	return float64(i)
}
func SummaryUniformN(t *testing.T, n int) {
	assert := assert.New(t)
	vals, _ := GenSummary(n, UniformGenerator)

	for i, v := range vals {
		var exp float64
		if testQuantiles[i] == 0 {
			exp = 0
		} else if testQuantiles[i] == 1 {
			exp = float64(n) - 1
		} else {
			rank := math.Ceil(testQuantiles[i] * float64(n))
			exp = rank - 1
		}
		assert.InDelta(exp, v, EPSILON*float64(n), "quantile %f failed, exp: %f, val: %f", testQuantiles[i], exp, v)
	}
}
func TestSummaryUniform10(t *testing.T) {
	SummaryUniformN(t, 10)
}
func TestSummaryUniform100(t *testing.T) {
	SummaryUniformN(t, 100)
}
func TestSummaryUniform1000(t *testing.T) {
	SummaryUniformN(t, 1000)
}
func TestSummaryUniform10000(t *testing.T) {
	SummaryUniformN(t, 10000)
}
func TestSummaryUniform100000(t *testing.T) {
	SummaryUniformN(t, 100000)
}

func NewSummaryWithTestData() *Summary {
	s := NewSummary()

	for i := 0; i < 1000; i++ {
		s.Insert(float64(i), uint64(i))
	}

	return s
}

func TestSummaryMerge(t *testing.T) {
	assert := assert.New(t)
	s1 := NewSummary()
	for i := 0; i < 101; i++ {
		s1.Insert(float64(i), uint64(i))
	}

	s2 := NewSummary()
	for i := 0; i < 50; i++ {
		s2.Insert(float64(i), uint64(i))
	}

	s1.Merge(s2)

	expected := map[float64]int{
		0.0: 0,
		0.2: 15,
		0.4: 30,
		0.6: 45,
		0.8: 70,
		1.0: 100,
	}

	for q, e := range expected {
		v, _ := s1.Quantile(q)
		assert.Equal(e, v)
	}
}

func TestSummaryMergeReal(t *testing.T) {
	s := NewSummary()
	for n := 0; n < 10000; n++ {
		s1 := NewSummary()
		for i := 0; i < 100; i++ {
			s1.Insert(float64(i), uint64(i))
		}
		s.Merge(s1)

	}

	fmt.Println(s)
	slices := s.BySlices(0)
	fmt.Println(slices)
	total := 0
	for _, s := range slices {
		total += s.Weight
	}
	fmt.Println(total)
}

func TestSummaryMergeInsertion(t *testing.T) {
	s1 := NewSummary()
	for n := 0; n < 1000; n++ {
		for i := 0; i < 100; i++ {
			s1.Insert(float64(i), uint64(i))
		}
	}

	fmt.Println(s1)
	slices := s1.BySlices(0)
	fmt.Println(slices)
	total := 0
	for _, s := range slices {
		total += s.Weight
	}
	fmt.Println(total)
}

func TestSummaryBySlices(t *testing.T) {
	assert := assert.New(t)

	s := NewSummary()
	for i := 1; i < 11; i++ {
		s.Insert(float64(i), uint64(i))
	}
	s.Insert(float64(5), uint64(42))
	s.Insert(float64(5), uint64(53))

	slices := s.BySlices(0)
	fmt.Println(slices)
	assert.Equal(10, len(slices))
	for i, sl := range slices {
		assert.Equal(i+1, sl.Start)
		assert.Equal(i+1, sl.End)
		if i == 4 {
			assert.Equal(3, sl.Weight)
		} else {
			assert.Equal(1, sl.Weight)
		}
	}
}

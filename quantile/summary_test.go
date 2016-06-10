package quantile

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

/************************************************************************************
	DATA VALIDATION, with different strategies make sure of the correctness of
	our epsilon-approximate quantiles
************************************************************************************/

var testQuantiles = []float64{0, 0.1, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 0.999, 0.9999, 1}

func GenSummarySkiplist(n int, gen func(i int) float64) ([]float64, []uint64) {
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

func GenSummarySlice(n int, gen func(i int) float64) ([]float64, []uint64) {
	s := NewSliceSummary()

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
func SummarySkiplistConstantN(t *testing.T, n int) {
	assert := assert.New(t)
	vals, _ := GenSummarySkiplist(n, ConstantGenerator)
	for _, v := range vals {
		assert.Equal(42, v)
	}
}
func SummarySliceConstantN(t *testing.T, n int) {
	assert := assert.New(t)
	vals, _ := GenSummarySlice(n, ConstantGenerator)
	for _, v := range vals {
		assert.Equal(42, v)
	}
}
func TestSummarySkiplistConstant10(t *testing.T) {
	SummarySkiplistConstantN(t, 10)
}
func TestSummarySkiplistConstant100(t *testing.T) {
	SummarySkiplistConstantN(t, 100)
}
func TestSummarySkiplistConstant1000(t *testing.T) {
	SummarySkiplistConstantN(t, 1000)
}
func TestSummarySkiplistConstant10000(t *testing.T) {
	SummarySkiplistConstantN(t, 10000)
}
func TestSummarySkiplistConstant100000(t *testing.T) {
	SummarySkiplistConstantN(t, 100000)
}
func TestSummarySliceConstant10(t *testing.T) {
	SummarySliceConstantN(t, 10)
}
func TestSummarySliceConstant100(t *testing.T) {
	SummarySliceConstantN(t, 100)
}
func TestSummarySliceConstant1000(t *testing.T) {
	SummarySliceConstantN(t, 1000)
}
func TestSummarySliceConstant10000(t *testing.T) {
	SummarySliceConstantN(t, 10000)
}
func TestSummarySliceConstant100000(t *testing.T) {
	SummarySliceConstantN(t, 100000)
}

/* uniform distribution
   expected quantiles are easily to compute as the value == its rank
   1 to i
*/
func UniformGenerator(i int) float64 {
	return float64(i)
}
func SummarySkiplistUniformN(t *testing.T, n int) {
	assert := assert.New(t)
	vals, _ := GenSummarySkiplist(n, UniformGenerator)

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
func SummarySliceUniformN(t *testing.T, n int) {
	assert := assert.New(t)
	vals, _ := GenSummarySlice(n, UniformGenerator)

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
func TestSummarySkiplistUniform10(t *testing.T) {
	SummarySkiplistUniformN(t, 10)
}
func TestSummarySkiplistUniform100(t *testing.T) {
	SummarySkiplistUniformN(t, 100)
}
func TestSummarySkiplistUniform1000(t *testing.T) {
	SummarySkiplistUniformN(t, 1000)
}
func TestSummarySkiplistUniform10000(t *testing.T) {
	SummarySkiplistUniformN(t, 10000)
}
func TestSummarySkiplistUniform100000(t *testing.T) {
	SummarySkiplistUniformN(t, 100000)
}
func TestSummarySliceUniform10(t *testing.T) {
	SummarySliceUniformN(t, 10)
}
func TestSummarySliceUniform100(t *testing.T) {
	SummarySliceUniformN(t, 100)
}
func TestSummarySliceUniform1000(t *testing.T) {
	SummarySliceUniformN(t, 1000)
}
func TestSummarySliceUniform10000(t *testing.T) {
	SummarySliceUniformN(t, 10000)
}
func TestSummarySliceUniform100000(t *testing.T) {
	SummarySliceUniformN(t, 100000)
}

func NewSummaryWithTestData() *Summary {
	s := NewSummary()

	for i := 0; i < 1000; i++ {
		s.Insert(float64(i), uint64(i))
	}

	return s
}

func TestSummaryGob(t *testing.T) {
	assert := assert.New(t)

	s := NewSummaryWithTestData()
	bytes, err := s.GobEncode()
	assert.Nil(err)
	ss := NewSummary()
	ss.GobDecode(bytes)

	assert.Equal(s.N, ss.N)
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

func TestSummarySliceMerge(t *testing.T) {
	assert := assert.New(t)
	s1 := NewSliceSummary()
	for i := 0; i < 101; i++ {
		s1.Insert(float64(i), uint64(i))
	}

	s2 := NewSliceSummary()
	for i := 0; i < 50; i++ {
		s2.Insert(float64(i), uint64(i))
	}

	s1.Merge(s2)

	expected := map[float64]int{
		0.0: 0,
		0.2: 15,
		0.4: 30,
		0.6: 45,
		0.8: 71, // should be 70 ? FIXME[leo]
		1.0: 100,
	}

	for q, e := range expected {
		v, _ := s1.Quantile(q)
		assert.Equal(e, v)
	}
}

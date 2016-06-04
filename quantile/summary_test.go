package quantile

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var TestArray = [...]float64{9193603307515863, 6754367334490737, 3319615827007736, 5889610372829623, 9056711574806808, 552001800400511, 13400109297856412, 9290419307253104, 22335189735651727, 20198755397426237, 6750759924055697, 5282418092338629, 7669954555439878, 9871073458267054, 9596813405908124, 17826985234078452, 20809685535785593, 169504538801585, 3742572289274816, 5269690153609658, 5911223635170287, 8307883610131084, 2782048181817819, 3730609951306336, 4049109332077119, 25526756656192197, 9178098692976052, 14247068431339682, 9790147915267598, 26879545051663945, 6061096718141231, 18513304861202883, 36038916238811566, 25585683826277574, 9896041109367753, 6071196089481084, 7192619690997925, 4638251828961559, 3042642059231756, 4695726824572235, 44887308998594665, 3933519497422833, 17471932249059073, 4220651987288255, 6037514377711865, 9334562578999125, 1564928248590489, 3263044748940436, 4590412317155934, 5470186478741421, 7945869119417566, 5954581154568589, 894968515670638, 7620414993900439, 9670709128536947, 6566981438432174, 7097246010376681, 38073653883035684, 28936241604793246, 9572137207232106, 8763202610627523, 24879815234751902, 29520475378617226, 4325698909679998, 1385395600985416, 21587377639944705, 23838639891325397, 47092556792851724, 380708162070026, 7041179813266126, 11313259703460376, 16338073267858633, 7157789354354156, 7679321360662391, 1296853853173733, 8289135153494715, 5067591593829185, 6875574194262736, 6816612766339816, 2744231634144031, 3371312301999041, 3582026995577731, 9560887595174091, 9478826709460911, 14554548042944748, 2381196435702264, 920004969814432, 16786031463171992, 9843289339356544, 4813972590910607, 8148484660815154, 3645114654798244, 57439089131166135, 8901361618305092, 1776157835648735, 5659433639754501, 43703315824233925, 760807861832865, 4575552020357635, 29239935027624495}

/*
FIXME? Right now we're implementing the "lower" interpolation which is not the "linear" default one.

In [7]: np.percentile(TestArray, 50, interpolation='lower')
Out[7]: 7192619690997925

In [8]: np.percentile(TestArray, 95, interpolation='lower')
Out[8]: 36038916238811566

In [9]: np.percentile(TestArray, 99, interpolation='lower')
Out[9]: 47092556792851724

In [10]: np.percentile(TestArray, 99.9, interpolation='lower')
Out[10]: 47092556792851724
*/

// NewSummaryWithTestData returns the Summary
func NewSummaryWithTestData() *Summary {
	s := NewSummary()

	for i, v := range TestArray {
		s.Insert(v, uint64(i))
	}

	return s
}

func TestSummaryInsertion(t *testing.T) {
	assert := assert.New(t)

	s := NewSummaryWithTestData()
	assert.Equal(100, s.N)
}

type Quantile struct {
	Value   float64
	Samples []uint64
}

func TestSummaryQuantile(t *testing.T) {
	assert := assert.New(t)

	// For 100 elts and eps=0.01 the error on the rank is 0.01 * 100 = 1
	// So we can have these results:
	// *  7157789354354156, (SID=72)
	// *  7192619690997925, (SID=36)
	// *  7620414993900439, (SID=53)
	s := NewSummaryWithTestData()

	v, samples := s.Quantile(0.5)
	// our sample array only yields a sample per value
	assert.Equal(1, len(samples))
	acceptable := []Quantile{
		Quantile{Value: 7157789354354156, Samples: []uint64{72}},
		Quantile{Value: 7192619690997925, Samples: []uint64{36}},
		Quantile{Value: 7620414993900439, Samples: []uint64{53}},
	}
	foundCorrectQuantile := false
	for _, q := range acceptable {
		foundCorrectQuantile = q.Value == v && q.Samples[0] == samples[0]
		if foundCorrectQuantile {
			break
		}
	}

	assert.True(foundCorrectQuantile, "Quantile %d (samples=%v) not found", v, samples)
}

func TestSummaryMarshal(t *testing.T) {
	assert := assert.New(t)

	s := NewSummaryWithTestData()

	b, err := json.Marshal(s)
	assert.Nil(err)

	// Now test contents
	ss := Summary{}
	err = json.Unmarshal(b, &ss)

	assert.Equal(s.N, ss.N)
	v1, samp1 := s.Quantile(0.5)
	v2, samp2 := ss.Quantile(0.5)

	assert.Equal(v1, v2)
	assert.Equal(1, len(samp1))
	assert.Equal(1, len(samp2))

	// Verify samples are correct
	samp1Correct := false
	for i, val := range TestArray {
		if val == v1 && samp1[0] == uint64(i) {
			samp1Correct = true
			break
		}
	}
	assert.True(samp1Correct, "1: sample %v incorrect for quantile %d", samp1, v1)

	samp2Correct := false
	for i, val := range TestArray {
		if val == v2 && samp2[0] == uint64(i) {
			samp2Correct = true
			break
		}
	}
	assert.True(samp2Correct, "2: sample %v incorrect for quantile %d", samp2, v2)
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

// TestSummaryNonZeroMerge ensures we can safely merge samples with non-zero
// values (these were previously causing a bug).
func TestSummaryNonZeroMerge(t *testing.T) {
	assert := assert.New(t)
	s1 := NewSummary()
	s2 := NewSummary()
	for i := 1; i < 6; i++ {
		s1.Insert(float64(i), uint64(i))
		s2.Insert(float64(i), uint64(i))
	}
	s1.Merge(s2)

	v, _ := s1.Quantile(0)
	assert.Equal(v, 1)
	v, _ = s1.Quantile(0.5)
	assert.Equal(v, 3)
	v, _ = s1.Quantile(1)
	assert.Equal(v, 5)
}

func TestSummaryBySlices(t *testing.T) {
	// Write a test!
	t.Skip()
}

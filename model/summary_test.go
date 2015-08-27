package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var TestArray = [...]int64{9193603307515863, 6754367334490737, 3319615827007736, 5889610372829623, 9056711574806808, 552001800400511, 13400109297856412, 9290419307253104, 22335189735651727, 20198755397426237, 6750759924055697, 5282418092338629, 7669954555439878, 9871073458267054, 9596813405908124, 17826985234078452, 20809685535785593, 169504538801585, 3742572289274816, 5269690153609658, 5911223635170287, 8307883610131084, 2782048181817819, 3730609951306336, 4049109332077119, 25526756656192197, 9178098692976052, 14247068431339682, 9790147915267598, 26879545051663945, 6061096718141231, 18513304861202883, 36038916238811566, 25585683826277574, 9896041109367753, 6071196089481084, 7192619690997925, 4638251828961559, 3042642059231756, 4695726824572235, 44887308998594665, 3933519497422833, 17471932249059073, 4220651987288255, 6037514377711865, 9334562578999125, 1564928248590489, 3263044748940436, 4590412317155934, 5470186478741421, 7945869119417566, 5954581154568589, 894968515670638, 7620414993900439, 9670709128536947, 6566981438432174, 7097246010376681, 38073653883035684, 28936241604793246, 9572137207232106, 8763202610627523, 24879815234751902, 29520475378617226, 4325698909679998, 1385395600985416, 21587377639944705, 23838639891325397, 47092556792851724, 380708162070026, 7041179813266126, 11313259703460376, 16338073267858633, 7157789354354156, 7679321360662391, 1296853853173733, 8289135153494715, 5067591593829185, 6875574194262736, 6816612766339816, 2744231634144031, 3371312301999041, 3582026995577731, 9560887595174091, 9478826709460911, 14554548042944748, 2381196435702264, 920004969814432, 16786031463171992, 9843289339356544, 4813972590910607, 8148484660815154, 3645114654798244, 57439089131166135, 8901361618305092, 1776157835648735, 5659433639754501, 43703315824233925, 760807861832865, 4575552020357635, 29239935027624495}

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

func TestNewSummary(t *testing.T) {
	assert := assert.New(t)
	d := NewSummary(0)

	_, isExact := d.(*ExactSummary)
	assert.True(isExact)
	_, isGK := d.(*GKSummary)
	assert.False(isGK)

	d = NewSummary(0.01)

	_, isExact = d.(*ExactSummary)
	assert.False(isExact)
	_, isGK = d.(*GKSummary)
	assert.True(isGK)
}

// NewSummaryWithTestData returns the Summary
func NewSummaryWithTestData(eps float64) Summary {
	d := NewSummary(eps)

	for i, v := range TestArray {
		d.Insert(v, uint64(i))
	}

	return d
}

func TestExactSummaryInsertion(t *testing.T) {
	assert := assert.New(t)

	d := NewSummaryWithTestData(0)

	// struct should have seen as many samples so far
	assert.Equal(100, d.(*ExactSummary).n)
	assert.Equal(100, len(d.(*ExactSummary).data))
}

func TestExactSummaryQuantile(t *testing.T) {
	assert := assert.New(t)

	d := NewSummaryWithTestData(0)

	v, spans := d.Quantile(0.5)
	assert.Equal(7192619690997925, v)
	assert.Equal(1, len(spans))
	assert.Equal(36, int(spans[0]))
}

func TestGKSummaryInsertion(t *testing.T) {
	assert := assert.New(t)

	d := NewSummaryWithTestData(0.01)
	assert.Equal(100, d.(*GKSummary).N)
}

func TestGKSummaryQuantile(t *testing.T) {
	assert := assert.New(t)

	// For 100 elts and eps=0.01 the error on the rank is 0.01 * 100 = 1
	// So we can have these results:
	// *  7157789354354156, (SID=72)
	// *  7192619690997925, (SID=36)
	// *  7620414993900439, (SID=53)
	d := NewSummaryWithTestData(0.01)

	// FIXME: assert the returned sample SID
	v, _ := d.Quantile(0.5)
	acceptable := []int64{7157789354354156, 7192619690997925, 7620414993900439}
	assert.Contains(acceptable, v)
}

func TestGKSummaryInsertionHugeScale(t *testing.T) {
	assert := assert.New(t)
	repet := 10

	d := NewSummary(0.001)
	for i := 0; i < repet*len(TestArray); i++ {
		d.Insert(TestArray[i%len(TestArray)], uint64(i))
	}

	/* to print the skiplist, DEBUG ONLY!
	seen := make(map[uintptr]bool)
	d.(*GKSummary).summary.head.Println(0, &seen)
	*/

	assert.Equal(repet*len(TestArray), d.(*GKSummary).N)
	// FIXME: assert correctness of this, should return proper quantiles
}

func TestGKSummaryMarshal(t *testing.T) {
	assert := assert.New(t)

	d := NewSummaryWithTestData(0.01)

	// FIXME: test the real data in it
	assert.NotPanics(func() { d.(*GKSummary).Encode() })
}

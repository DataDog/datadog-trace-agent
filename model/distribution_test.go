package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var TestArray = [...]float64{91.93603307515863, 675.4367334490737, 331.9615827007736, 58.89610372829623, 9.056711574806808, 552.001800400511, 134.00109297856412, 929.0419307253104, 223.35189735651727, 201.98755397426237, 675.0759924055697, 528.2418092338629, 766.9954555439878, 98.71073458267054, 959.6813405908124, 178.26985234078452, 208.09685535785593, 169.504538801585, 374.2572289274816, 526.9690153609658, 591.1223635170287, 830.7883610131084, 278.2048181817819, 373.0609951306336, 404.9109332077119, 255.26756656192197, 917.8098692976052, 142.47068431339682, 979.0147915267598, 268.79545051663945, 606.1096718141231, 185.13304861202883, 360.38916238811566, 25.585683826277574, 989.6041109367753, 607.1196089481084, 719.2619690997925, 463.8251828961559, 304.2642059231756, 469.5726824572235, 448.87308998594665, 393.3519497422833, 17.471932249059073, 422.0651987288255, 603.7514377711865, 933.4562578999125, 156.4928248590489, 32.63044748940436, 459.0412317155934, 547.0186478741421, 794.5869119417566, 595.4581154568589, 894.968515670638, 762.0414993900439, 96.70709128536947, 656.6981438432174, 709.7246010376681, 380.73653883035684, 28.936241604793246, 957.2137207232106, 876.3202610627523, 248.79815234751902, 295.20475378617226, 432.5698909679998, 138.5395600985416, 215.87377639944705, 238.38639891325397, 470.92556792851724, 380.708162070026, 704.1179813266126, 113.13259703460376, 1.6338073267858633, 715.7789354354156, 767.9321360662391, 129.6853853173733, 828.9135153494715, 506.7591593829185, 687.5574194262736, 681.6612766339816, 274.4231634144031, 33.71312301999041, 358.2026995577731, 956.0887595174091, 947.8826709460911, 145.54548042944748, 238.1196435702264, 920.004969814432, 167.86031463171992, 984.3289339356544, 481.3972590910607, 814.8484660815154, 364.5114654798244, 57.439089131166135, 890.1361618305092, 177.6157835648735, 565.9433639754501, 43.703315824233925, 760.807861832865, 457.5552020357635, 292.39935027624495}

/*
FIXME? Right now we're implementing the "lower" interpolation which is not the "linear" default one.

In [7]: np.percentile(TestArray, 50, interpolation='lower')
Out[7]: 422.06519872882552

In [8]: np.percentile(TestArray, 95, interpolation='lower')
Out[8]: 956.08875951740913

In [9]: np.percentile(TestArray, 99, interpolation='lower')
Out[9]: 984.32893393565439

In [10]: np.percentile(TestArray, 99.9, interpolation='lower')
Out[10]: 984.32893393565439
*/

func TestNewDistribution(t *testing.T) {
	assert := assert.New(t)
	d := NewDistribution(0)

	_, isExact := d.(*ExactDistro)
	assert.True(isExact)
	_, isGK := d.(*GKDistro)
	assert.False(isGK)

	d = NewDistribution(0.01)

	_, isExact = d.(*ExactDistro)
	assert.False(isExact)
	_, isGK = d.(*GKDistro)
	assert.True(isGK)
}

// NewDistributionWithTestData returns the Distribution + an array to show which samples were kept
func NewDistributionWithTestData(eps float64) (Distribution, []bool) {
	d := NewDistribution(eps)
	k := make([]bool, len(TestArray))

	for i, v := range TestArray {
		k[i] = d.Insert(v, TID(i))
	}

	return d, k
}

func TestExactDistributionInsertion(t *testing.T) {
	assert := assert.New(t)

	d, _ := NewDistributionWithTestData(0)

	// struct should have seen as many samples so far
	assert.Equal(100, d.(*ExactDistro).n)
	assert.Equal(100, len(d.(*ExactDistro).summary))
}

func TestExactDistributionQuantile(t *testing.T) {
	assert := assert.New(t)

	d, _ := NewDistributionWithTestData(0)

	v, spans := d.Quantile(0.5)
	assert.Equal(422.06519872882552, v)
	assert.Equal(1, len(spans))
	assert.Equal(43, int(spans[0]))
}

func TestExactDistributionDropTraces(t *testing.T) {
	assert := assert.New(t)

	d := NewDistribution(0)
	kept := d.Insert(42.42, TID(1))
	assert.True(kept)

	// the ExactDistro drops subsequent same traces (DUMB)
	kept = d.Insert(42.42, TID(2))
	assert.False(kept)
}

func TestGKDistributionInsertion(t *testing.T) {
	assert := assert.New(t)

	d, _ := NewDistributionWithTestData(0.01)
	assert.Equal(100, d.(*GKDistro).n)
}

func TestGKDistributionQuantile(t *testing.T) {
	assert := assert.New(t)

	// For 100 elts and eps=0.01 the error on the rank is 0.01 * 100 = 1
	// So we can have these results:
	// *  404.9109332077119, (TID=24)
	// *  422.0651987288255, (TID=43)
	// *  432.5698909679998, (TID=63)
	d, _ := NewDistributionWithTestData(0.01)

	// FIXME: assert the returned sample TID
	v, _ := d.Quantile(0.5)
	acceptable := []float64{404.9109332077119, 422.0651987288255, 432.5698909679998}
	assert.Contains(acceptable, v)
}

func TestGKDistributionInsertionHugeScale(t *testing.T) {
	assert := assert.New(t)
	repet := 10

	d := NewDistribution(0.001)
	for i := 0; i < repet*len(TestArray); i++ {
		d.Insert(TestArray[i%len(TestArray)], TID(i))
	}

	/* to print the skiplist, DEBUG ONLY!
	seen := make(map[uintptr]bool)
	d.(*GKDistro).summary.head.Println(0, &seen)
	*/

	assert.Equal(repet*len(TestArray), d.(*GKDistro).n)
	// FIXME: assert correctness of this, should return proper quantiles
}

func TestGKDistributionMarshall(t *testing.T) {
	assert := assert.New(t)

	d, _ := NewDistributionWithTestData(0.01)

	// FIXME: test the real data in it
	assert.NotPanics(func() { d.(*GKDistro).summary.Marshal() })
}

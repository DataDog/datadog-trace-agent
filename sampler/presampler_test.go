package sampler

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalcPreSampleRate(t *testing.T) {
	assert := assert.New(t)

	expected := map[[3]float64]float64{
		[3]float64{0.1, 0.1, 1}:     1,                   // just at max CPU usage, currently not sampling
		[3]float64{0.2, 0.1, 1}:     1,                   // below max CPU usage, currently not sampling
		[3]float64{0.1, 0.15, 1}:    0.6153846153846154,  // 150% of max CPU usage, currently not sampling -> sample below 66%
		[3]float64{0.1, 0.2, 1}:     0.4444444444444444,  // 200% of max CPU usage, currently not sampling -> sample below 50%
		[3]float64{0.2, 1, 1}:       0.18367346938775514, // 500% of max CPU usage, currently not sampling -> sample below 20%
		[3]float64{0.1, 0.11, 1}:    1,                   // 110% of max CPU usage, currently not sampling
		[3]float64{0.1, 0.09, 1}:    1,                   // 90% of max CPU usage, currently not sampling
		[3]float64{0.1, 0.05, 1}:    1,                   // 50% of max CPU usage, currently not sampling
		[3]float64{0.1, 0.11, 0.5}:  0.5,                 // 110% of max CPU usage, currently sampling at 50% -> keep going
		[3]float64{0.1, 0.09, 0.5}:  0.5714285714285715,  // 90% of max CPU usage, currently sampling at 50% -> sample above 50%
		[3]float64{0.1, 0.5, 0.5}:   0.08333333333333334, // 50% of max CPU usage, currently sampling at 50% -> sample less
		[3]float64{0.15, 0.05, 0.5}: 1,                   // 33% of max CPU usage, currently sampling at 50% -> back to no sampling
		[3]float64{0.1, 1000000, 1}: 0.05,                // insane CPU usage, currently sampling at 50% -> return min
		[3]float64{0.1, 0.05, 0.1}:  0.26666666666666666, // 50% of max CPU, currently sampling at 10% -> double the rate
		[3]float64{0.04, 0.05, 1}:   0.6666666666666666,  // 4% of max CPU, currently not sampling -> sampling at 66%
		[3]float64{0.025, 0.05, 1}:  0.6666666666666666,  // 2,5% of max CPU, currently not sampling -> same rate than with 4%
		[3]float64{0.01, 0.05, 0.1}: 1,                   // non-sensible max CPU -> disable pre-sampling
		[3]float64{0.1, 0, 0.1}:     1,                   // non-sensible current CPU usage -> disable pre-sampling
		[3]float64{0.1, 0.05, 0}:    1,                   // non-sensible current rate -> disable pre-sampling
	}

	for k, v := range expected {
		r := CalcPreSampleRate(k[0], k[1], k[2])
		assert.Equal(v, r, "bad pre sample rate for maxUserAvg=%f currentUserAvg=%f, currentRate=%f, got %v, expected %v", k[0], k[1], k[2], r, v)
	}
}

type testLogger struct{}

func (*testLogger) Errorf(format string, params ...interface{}) {}

func newTestLogger() *testLogger { return &testLogger{} }

func TestPreSamplerRace(t *testing.T) {
	var wg sync.WaitGroup

	const N = 1000
	ps := NewPreSampler(1.0, newTestLogger())
	wg.Add(5)

	go func() {
		for i := 0; i < N; i++ {
			ps.SetRate(0.5)
			time.Sleep(time.Microsecond)
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < N; i++ {
			_ = ps.Rate()
			time.Sleep(time.Microsecond)
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < N; i++ {
			_ = ps.RealRate()
			time.Sleep(time.Microsecond)
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < N; i++ {
			_ = ps.sampleWithCount(42)
			time.Sleep(time.Microsecond)
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < N; i++ {
			ps.DecayScore()
			time.Sleep(time.Microsecond)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestPreSamplerSampleWithCount(t *testing.T) {
	assert := assert.New(t)

	ps := NewPreSampler(1.0, newTestLogger())
	ps.SetRate(0.2)
	assert.Equal(0.2, ps.RealRate(), "by default, RealRate returns wished rate")
	assert.True(ps.sampleWithCount(100), "always accept first payload")
	ps.DecayScore()
	assert.False(ps.sampleWithCount(10), "refuse as this accepting this would make 100%")
	ps.DecayScore()
	assert.Equal(0.898876404494382, ps.RealRate())
	assert.False(ps.sampleWithCount(290), "still refuse")
	ps.DecayScore()
	assert.False(ps.sampleWithCount(99), "just below the limit")
	ps.DecayScore()
	assert.True(ps.sampleWithCount(1), "should there be no decay, this one would be dropped, but with decay, the rate decreased as the recently dropped gain importance over the old initially accepted")
	ps.DecayScore()
	assert.Equal(0.16365162139216005, ps.RealRate(), "well below 20%, again, decay speaks")
	assert.True(ps.sampleWithCount(1000000), "accepting payload with many traces")
	ps.DecayScore()
	assert.Equal(0.9997119577953764, ps.RealRate(), "real rate is almost 1, as we accepted a hudge payload")
	assert.False(ps.sampleWithCount(100000), "rejecting, real rate is too high now")
	ps.DecayScore()
	assert.Equal(0.8986487877795845, ps.RealRate(), "real rate should be now around 90%")
	assert.Equal(PreSamplerStats{
		Rate:                0.2,
		RecentPayloadsSeen:  4.492300911839488, // seen more than this... but decay in action
		RecentTracesSeen:    879284.5615616576,
		RecentTracesDropped: 89116.55620097058,
	}, ps.stats)
}

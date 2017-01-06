package watchdog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testDuration = time.Second
)

func TestCPULow(t *testing.T) {
	assert := assert.New(t)

	c := CPU()
	time.Sleep(testDuration)
	c = CPU()
	t.Logf("CPU (sleep): %v", c)

	// checking that CPU is low enough, this is theorically flaky,
	// but eating 50% of CPU for a time.Sleep is still not likely to happen often
	assert.Condition(func() bool { return c.UserAvg >= 0.0 }, "cpu avg should be positive")
	assert.Condition(func() bool { return c.UserAvg <= 0.5 }, "cpu avg should be below 0.5")
}

func doTestCPUHigh(t *testing.T, n int) {
	assert := assert.New(t)
	done := make(chan struct{}, 1)
	c := CPU()
	for i := 0; i < n; i++ {
		go func() {
			j := 0
			for {
				select {
				case <-done:
					return
				default:
					j++
				}
			}
		}()
	}
	time.Sleep(testDuration)
	c = CPU()
	for i := 0; i < n; i++ {
		done <- struct{}{}
	}
	t.Logf("CPU (%d goroutines): %v", n, c)

	// Checking that CPU is high enough, a very simple ++ loop should be
	// enough to stimulate one core and make it over 50%. One of the goals
	// of this test is to check that values are not wrong by a factor 100, such
	// as mismatching percentages and [0...1]  values.
	assert.Condition(func() bool { return c.UserAvg >= 0.5 }, "cpu avg is too low")
	assert.Condition(func() bool { return c.UserAvg <= float64(n+1) }, "cpu avg is too high")
}

func TestCPUHigh(t *testing.T) {
	doTestCPUHigh(t, 1)
	if testing.Short() {
		return
	}
	doTestCPUHigh(t, 10)
	doTestCPUHigh(t, 100)
}

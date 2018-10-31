package event

import (
	"math/rand"
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMaxEPSSampler(t *testing.T) {
	testEvents := generateTestEvents(1000, 0)
	testEventsSampledTraces := generateTestEvents(1000, 100)

	for name, testCase := range map[string]struct {
		events             []*model.APMEvent
		maxEPS             float64
		pastEPS            float64
		expectedSampledPct float64
		deltaPct           float64
	}{
		"low EPS":      {testEvents, 100, 50, 1., 0},
		"limit EPS":    {testEvents, 100, 100, 1., 0},
		"overload EPS": {testEvents, 100, 150, 100. / 150., 0.05},
		// We should always keep events for sampled traces even if we are above maxEPS
		"overload EPS - sampled": {testEventsSampledTraces, 100, 500, 1., 0},
		// We should always keep events for sampled traces even if we are above maxEPS
		"nil events": {make([]*model.APMEvent, 5), 100, 0, 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			counter := &MockRateCounter{
				GetRateResult: testCase.pastEPS,
			}

			sampler := newMaxEPSSampler(testCase.maxEPS, counter)
			sampler.Start()

			sampled := 0

			for _, event := range testCase.events {
				if sampler.Sample(event) {
					assert.EqualValues(testCase.expectedSampledPct, event.GetEventSamplerSampleRate())
					sampled++
				}
			}

			sampler.Stop()

			assert.InDelta(testCase.expectedSampledPct, float64(sampled)/float64(len(testEvents)), testCase.expectedSampledPct*testCase.deltaPct)

			nonNilEvents := 0
			nonTraceSampledEvents := 0

			for _, e := range testCase.events {
				if e != nil {
					nonNilEvents++

					if !e.TraceSampled {
						nonTraceSampledEvents++
					}
				}
			}

			assert.EqualValues(nonNilEvents, counter.CountCalls)
			assert.EqualValues(nonTraceSampledEvents, counter.GetRateCalls)
		})
	}
}

func generateTestEvents(numEvents int, pctWithSampledTrace int32) []*model.APMEvent {
	testEvents := make([]*model.APMEvent, numEvents)
	for i, _ := range testEvents {
		testEvents[i] = &model.APMEvent{
			Span:         testutil.RandomSpan(),
			TraceSampled: rand.Int31n(100) < pctWithSampledTrace,
		}
	}

	return testEvents
}

type MockRateCounter struct {
	CountCalls    int
	GetRateCalls  int
	GetRateResult float64
}

func (mc *MockRateCounter) Start() {}
func (mc *MockRateCounter) Stop()  {}

func (mc *MockRateCounter) Count() {
	mc.CountCalls++
}

func (mc *MockRateCounter) GetRate() float64 {
	mc.GetRateCalls++
	return mc.GetRateResult
}

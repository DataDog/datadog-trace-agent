package event

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMaxEPSSampler(t *testing.T) {
	type testCase struct {
		maxEPS               float64
		pastEPS              float64
		expectedSamplingRate float64
		deltaPct             float64
	}

	testCases := map[string]testCase{
		"low EPS":      {100, 50, 1., 0},
		"limit EPS":    {100, 100, 1., 0},
		"overload EPS": {100, 150, 100. / 150., 0.05},
	}

	testEvents := make([]*model.APMEvent, 1000)
	for i, _ := range testEvents {
		testEvents[i] = &model.APMEvent{Span: testutil.RandomSpan()}
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			counter := &MockRateCounter{
				GetRateResult: testCase.pastEPS,
			}

			sampler := NewMaxEPSSampler(testCase.maxEPS, counter)

			sampled := 0

			for _, event := range testEvents {
				decision := sampler.Sample(event)

				assert.NotEqual(DecisionNone, decision)

				if decision == DecisionSample {
					sampled++
				}
			}

			assert.InDelta(testCase.expectedSamplingRate, float64(sampled)/float64(len(testEvents)), testCase.expectedSamplingRate*testCase.deltaPct)

			assert.EqualValues(len(testEvents), counter.CountCalls)
			assert.EqualValues(len(testEvents), counter.GetRateCalls)
		})
	}
}

type MockRateCounter struct {
	CountCalls    int
	GetRateCalls  int
	GetRateResult float64
}

func (mc *MockRateCounter) Count() {
	mc.CountCalls++
}

func (mc *MockRateCounter) GetRate() float64 {
	mc.GetRateCalls++
	return mc.GetRateResult
}

package event

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMaxEPSSampler(t *testing.T) {
	for _, testCase := range []struct {
		name               string
		events             []*model.Event
		maxEPS             float64
		pastEPS            float64
		expectedSampleRate float64
		deltaPct           float64
	}{
		{"low", generateTestEvents(1000), 100, 50, 1., 0},
		{"limit", generateTestEvents(1000), 100, 100, 1., 0},
		{"overload", generateTestEvents(1000), 100, 150, 100. / 150., 0.05},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert := assert.New(t)

			counter := &MockRateCounter{
				GetRateResult: testCase.pastEPS,
			}
			sampler := newMaxEPSSampler(testCase.maxEPS)
			sampler.rateCounter = counter
			sampler.Start()

			sampled := 0
			for _, event := range testCase.events {
				sample, rate := sampler.Sample(event)
				if sample {
					sampled++
				}
				assert.EqualValues(testCase.expectedSampleRate, rate)
			}

			sampler.Stop()

			assert.InDelta(testCase.expectedSampleRate, float64(sampled)/float64(len(testCase.events)), testCase.expectedSampleRate*testCase.deltaPct)

			// Ensure PriorityUserKeep events do not affect counters
			nonUserKeep := 0

			for _, event := range testCase.events {
				if event.Priority != model.PriorityUserKeep {
					nonUserKeep++
				}
			}

			assert.EqualValues(nonUserKeep, counter.GetRateCalls)
			assert.EqualValues(nonUserKeep, counter.CountCalls)
		})
	}
}

func generateTestEvents(numEvents int) []*model.Event {
	testEvents := make([]*model.Event, numEvents)
	for i, _ := range testEvents {
		testEvents[i] = &model.Event{
			Span: testutil.RandomSpan(),
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

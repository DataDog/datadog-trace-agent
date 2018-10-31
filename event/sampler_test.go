package event

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestBatchSampler(t *testing.T) {
	testDecider := func(event *model.APMEvent) bool {
		if event == nil {
			return false
		}
		return event.Span.TraceID%2 == 0
	}

	testEvents := generateTestEvents(100, 50)
	testEventsSingleton := generateTestEvents(1, 50)

	for name, testCase := range map[string]struct {
		events         []*model.APMEvent
		expectedEvents []*model.APMEvent
	}{
		"no events":       {nil, nil},
		"nil events":      {make([]*model.APMEvent, 100), nil},
		"single event":    {testEventsSingleton, sampledFilter(testEventsSingleton, testDecider)},
		"multiple events": {testEvents, sampledFilter(testEvents, testDecider)},
	} {
		t.Run(name, func(t *testing.T) {
			samplerDecisions := make(map[*model.APMEvent]bool)

			for _, event := range testCase.events {
				samplerDecisions[event] = testDecider(event)
			}

			sampler := &MockSampler{SampleResult: samplerDecisions}

			batch := NewBatchSampler(sampler)

			batch.Start()

			assert.ElementsMatch(t, testCase.expectedEvents, batch.Sample(testCase.events))

			batch.Stop()

			nonNilEvents := 0

			for _, e := range testCase.events {
				if e != nil {
					nonNilEvents++
				}
			}

			assert.EqualValues(t, nonNilEvents, sampler.SampleCalls)
		})
	}
}

func sampledFilter(events []*model.APMEvent, decider func(event *model.APMEvent) bool) []*model.APMEvent {
	result := make([]*model.APMEvent, 0, len(events))

	for _, event := range events {
		if decider(event) {
			result = append(result, event)
		}
	}

	return result
}

type MockSampler struct {
	SampleCalls  int
	SampleResult map[*model.APMEvent]bool
}

func (ms *MockSampler) Start() {}
func (ms *MockSampler) Stop()  {}
func (ms *MockSampler) Sample(event *model.APMEvent) bool {
	ms.SampleCalls++

	return ms.SampleResult[event]
}

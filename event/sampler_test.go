package event

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/testutil"
	"github.com/stretchr/testify/assert"
)

func TestBatchSampler(t *testing.T) {
	testDecider := func(event *model.APMEvent) SamplingDecision {
		if event == nil {
			return DecisionNone
		}
		return SamplingDecision(event.Span.TraceID % 3)
	}

	testEvents := make([]*model.APMEvent, 100)
	for i, _ := range testEvents {
		testEvents[i] = &model.APMEvent{Span: testutil.RandomSpan()}
	}

	testEventsSingleton := []*model.APMEvent{testEvents[0]}

	type testCase struct {
		events         []*model.APMEvent
		expectedEvents []*model.APMEvent
	}

	testCases := map[string]testCase{
		"no events":       {nil, nil},
		"nil events":      {make([]*model.APMEvent, 100), nil},
		"single event":    {testEventsSingleton, sampledFilter(testEventsSingleton, testDecider)},
		"multiple events": {testEvents, sampledFilter(testEvents, testDecider)},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			samplerDecisions := make(map[*model.APMEvent]SamplingDecision)

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

func sampledFilter(events []*model.APMEvent, decider func(event *model.APMEvent) SamplingDecision) []*model.APMEvent {
	result := make([]*model.APMEvent, 0, len(events))

	for _, event := range events {
		if decider(event) == DecisionSample {
			result = append(result, event)
		}
	}

	return result
}

type MockSampler struct {
	SampleCalls  int
	SampleResult map[*model.APMEvent]SamplingDecision
}

func (ms *MockSampler) Start() {}
func (ms *MockSampler) Stop()  {}
func (ms *MockSampler) Sample(event *model.APMEvent) SamplingDecision {
	ms.SampleCalls++

	return ms.SampleResult[event]
}

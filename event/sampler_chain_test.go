package event

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/testutil"
	"github.com/stretchr/testify/assert"
)

func TestChainSampler(t *testing.T) {
	type testCase struct {
		samplers         []Sampler
		expectedDecision SamplingDecision
	}

	testCases := map[string]testCase{
		"no samplers":    {nil, DecisionNone},
		"single sampler": {[]Sampler{&MockDecisionSampler{DecisionSample}}, DecisionSample},
		"multi sampler - no decision": {[]Sampler{
			&MockDecisionSampler{DecisionNone},
			&MockDecisionSampler{DecisionNone},
		}, DecisionNone},
		"multi sampler - sample": {[]Sampler{
			&MockDecisionSampler{DecisionNone},
			&MockDecisionSampler{DecisionSample},
		}, DecisionSample},
		"multi sampler - dont sample": {[]Sampler{
			&MockDecisionSampler{DecisionNone},
			&MockDecisionSampler{DecisionDontSample},
		}, DecisionDontSample},
		"multi sampler - shortcircuit sample": {[]Sampler{
			&MockDecisionSampler{DecisionSample},
			&MockDecisionSampler{DecisionDontSample},
		}, DecisionSample},
		"multi sampler - shortcircuit dont sample": {[]Sampler{
			&MockDecisionSampler{DecisionDontSample},
			&MockDecisionSampler{DecisionSample},
		}, DecisionDontSample},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			var callbackResult *SamplingDecision

			callback := func(decision SamplingDecision) {
				callbackResult = &decision
			}

			chain := NewSamplerChain(testCase.samplers, callback)

			testEvent := &model.APMEvent{Span: testutil.RandomSpan()}
			assert.EqualValues(testCase.expectedDecision, chain.Sample(testEvent))
			if assert.NotNil(callbackResult) {
				assert.EqualValues(testCase.expectedDecision, *callbackResult)
			}
		})
	}
}

func TestChainSampler_NilCallback(t *testing.T) {
	chain := NewSamplerChain([]Sampler{&MockDecisionSampler{DecisionSample}}, nil)
	testEvent := &model.APMEvent{Span: testutil.RandomSpan()}

	assert.NotPanics(t, func() {
		chain.Sample(testEvent)
	})
}

type MockDecisionSampler struct {
	Decision SamplingDecision
}

func (fd *MockDecisionSampler) Sample(event *model.APMEvent) SamplingDecision {
	return fd.Decision
}

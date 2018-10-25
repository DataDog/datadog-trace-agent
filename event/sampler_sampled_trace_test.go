package event

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/testutil"
	"github.com/stretchr/testify/assert"
)

func TestSampledTraceSampler(t *testing.T) {
	type testCase struct {
		event            *model.APMEvent
		expectedDecision SamplingDecision
	}

	testCases := map[string]testCase{
		"trace not sampled": {&model.APMEvent{Span: testutil.RandomSpan(), TraceSampled: false}, DecisionNone},
		"trace sampled":     {&model.APMEvent{Span: testutil.RandomSpan(), TraceSampled: true}, DecisionSample},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			sampler := NewSampledTraceSampler()

			assert.EqualValues(t, testCase.expectedDecision, sampler.Sample(testCase.event))
		})
	}
}

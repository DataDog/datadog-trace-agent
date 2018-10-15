package main

import (
	"fmt"
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/stretchr/testify/assert"
)

func createTrace(serviceName string, operationName string, hasPriority bool, priority int, tracerRate float64) processedTrace {
	ws := model.WeightedSpan{Span: &model.Span{Service: serviceName, Name: operationName}}
	if hasPriority {
		ws.SetMetric(sampler.SamplingPriorityKey, float64(priority))
	}
	if tracerRate >= 0 {
		ws.SetMetric(sampler.EventSampleRateKey, tracerRate)

	}
	wt := model.WeightedTrace{&ws}
	return processedTrace{WeightedTrace: wt, Root: ws.Span}
}

func TestTransactionSampler(t *testing.T) {
	assert := assert.New(t)

	config := make(map[string]map[string]float64)
	config["myService"] = make(map[string]float64)
	config["myService"]["myOperation"] = 1

	config["mySampledService"] = make(map[string]float64)
	config["mySampledService"]["myOperation"] = 0

	config["lowRateService"] = make(map[string]float64)
	config["lowRateService"]["myOperation"] = 0.0001

	tests := []struct {
		name             string
		trace            processedTrace
		expectedSampling bool
	}{
		// Test how we match the Agent config map.
		{"service_match/name_match", createTrace("myService", "myOperation", false, 0, -1), true},
		{"service_no_match/name_match", createTrace("otherService", "myOperation", false, 0, -1), false},
		{"service_match/name_no_match", createTrace("myService", "otherOperation", false, 0, -1), false},
		{"service_no_match/name_no_match", createTrace("otherService", "otherOperation", false, 0, -1), false},
		// Test how the priority impacts config from the Agent when rate set to 0.
		{"agent_match/rate_0/priority_none", createTrace("mySampledService", "myOperation", false, 0, -1), false},
		{"agent_match/rate_0/priority_0", createTrace("mySampledService", "myOperation", true, 0, -1), false},
		{"agent_match/rate_0/priority_1", createTrace("mySampledService", "myOperation", true, 1, -1), false},
		{"agent_match/rate_0/priority_2", createTrace("mySampledService", "myOperation", true, 2, -1), false},
		// Test how the priority impacts config from the Agent when rate set to 1.
		{"agent_match/rate_1/priority_none", createTrace("myService", "myOperation", false, 0, -1), true},
		{"agent_match/rate_1/priority_0", createTrace("myService", "myOperation", true, 0, -1), true},
		{"agent_match/rate_1/priority_1", createTrace("myService", "myOperation", true, 1, -1), true},
		{"agent_match/rate_1/priority_2", createTrace("myService", "myOperation", true, 2, -1), true},
		// Test how the priority impacts config from the span when rate set to 0.
		{"tag_match/rate_0/priority_none", createTrace("customService", "myOperation", false, 0, 0), false},
		{"tag_match/rate_0/priority_0", createTrace("customService", "myOperation", true, 0, 0), false},
		{"tag_match/rate_0/priority_1", createTrace("customService", "myOperation", true, 1, 0), false},
		{"tag_match/rate_0/priority_2", createTrace("customService", "myOperation", true, 2, 0), false},
		// Test how the priority impacts config from the span when rate set to 1.
		{"tag_match/rate_1/priority_none", createTrace("customService", "myOperation", false, 0, 1), true},
		{"tag_match/rate_1/priority_0", createTrace("customService", "myOperation", true, 0, 1), true},
		{"tag_match/rate_1/priority_1", createTrace("customService", "myOperation", true, 1, 1), true},
		{"tag_match/rate_1/priority_2", createTrace("customService", "myOperation", true, 2, 1), true},
		// Test that p2 priority ensures sampling.
		{"agent_match/rate_low/priority_2", createTrace("lowRateService", "myOperation", true, 2, -1), true},
		{"tag_match/rate_low/priority_2", createTrace("lowRateService", "myOperation", true, 2, 0.0001), true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := newTransactionSampler(config)
			analyzedSpans := ts.Extract(test.trace)

			if test.expectedSampling {
				assert.Len(analyzedSpans, 1, fmt.Sprintf("Trace %v should have been sampled", test.trace))
			} else {
				assert.Len(analyzedSpans, 0, fmt.Sprintf("Trace %v should not have been sampled", test.trace))
			}
		})
	}
}

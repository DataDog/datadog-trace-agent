package main

import (
	"fmt"
	"testing"

	"github.com/DataDog/datadog-trace-agent/agent"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/stretchr/testify/assert"
)

func createTrace(serviceName string, operationName string, topLevel bool, hasPriority bool, priority int) processedTrace {
	ws := agent.WeightedSpan{TopLevel: topLevel, Span: &agent.Span{Service: serviceName, Name: operationName}}
	if hasPriority {
		ws.SetMetric(sampler.SamplingPriorityKey, float64(priority))
	}
	wt := agent.WeightedTrace{&ws}
	return processedTrace{WeightedTrace: wt, Root: ws.Span}
}

func TestTransactionSampler(t *testing.T) {
	assert := assert.New(t)

	config := make(map[string]map[string]float64)
	config["myService"] = make(map[string]float64)
	config["myService"]["myOperation"] = 1

	config["mySampledService"] = make(map[string]float64)
	config["mySampledService"]["myOperation"] = 0

	tests := []struct {
		name             string
		trace            processedTrace
		expectedSampling bool
	}{
		{"Top-level service and span name match", createTrace("myService", "myOperation", true, false, 0), true},
		{"Top-level service name doesn't match", createTrace("otherService", "myOperation", true, false, 0), false},
		{"Top-level span name doesn't match", createTrace("myService", "otherOperation", true, false, 0), false},
		{"Top-level service and span name don't match", createTrace("otherService", "otherOperation", true, false, 0), false},
		{"Non top-level service and span name match", createTrace("myService", "myOperation", false, false, 0), true},
		{"Non top-level service name doesn't match", createTrace("otherService", "myOperation", false, false, 0), false},
		{"Non top-level span name doesn't match", createTrace("myService", "otherOperation", false, false, 0), false},
		{"Non top-level service and span name don't match", createTrace("otherService", "otherOperation", false, false, 0), false},
		{"Match, sampling rate 0, no priority", createTrace("mySampledService", "myOperation", true, false, 0), false},
		{"Match, sampling rate 0, priority 1", createTrace("mySampledService", "myOperation", true, true, 1), false},
		{"Match, sampling rate 0, priority 2", createTrace("mySampledService", "myOperation", true, true, 2), true},
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

package main

import (
	"fmt"
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func createTrace(serviceName string, operationName string, topLevel bool) processedTrace {
	ws := model.WeightedSpan{TopLevel: topLevel, Span: &model.Span{Service: serviceName, Name: operationName}}
	wt := model.WeightedTrace{&ws}
	return processedTrace{WeightedTrace: wt}
}

func TestTransactionSampler(t *testing.T) {
	assert := assert.New(t)

	config := make(map[string]map[string]float64)
	config["myService"] = make(map[string]float64)
	config["myService"]["myOperation"] = 1

	tests := []struct {
		name             string
		trace            processedTrace
		expectedSampling bool
	}{
		{"Top-level service and span name match", createTrace("myService", "myOperation", true), true},
		{"Top-level service name doesn't match", createTrace("otherService", "myOperation", true), false},
		{"Top-level span name doesn't match", createTrace("myService", "otherOperation", true), false},
		{"Top-level service and span name don't match", createTrace("otherService", "otherOperation", true), false},
		{"Non top-level service and span name match", createTrace("myService", "myOperation", false), true},
		{"Non top-level service name doesn't match", createTrace("otherService", "myOperation", false), false},
		{"Non top-level span name doesn't match", createTrace("myService", "otherOperation", false), false},
		{"Non top-level service and span name don't match", createTrace("otherService", "otherOperation", false), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzed := make(chan *model.Span, 1)
			ts := newTransactionSampler(config, analyzed)
			ts.Add(test.trace)
			close(analyzed)

			analyzedSpans := make([]*model.Span, 0)
			for s := range analyzed {
				analyzedSpans = append(analyzedSpans, s)
			}

			assert.True(ts.Enabled())
			if test.expectedSampling {
				assert.Len(analyzedSpans, 1, fmt.Sprintf("Trace %v should have been sampled", test.trace))
			} else {
				assert.Len(analyzedSpans, 0, fmt.Sprintf("Trace %v should not have been sampled", test.trace))
			}
		})
	}
}

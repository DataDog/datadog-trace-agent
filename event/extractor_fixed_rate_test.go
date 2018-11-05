package event

import (
	"math/rand"
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
)

func createTestSpans(serviceName string, operationName string) []*model.WeightedSpan {
	spans := make([]*model.WeightedSpan, 1000)
	for i, _ := range spans {
		spans[i] = &model.WeightedSpan{Span: &model.Span{TraceID: rand.Uint64(), Service: serviceName, Name: operationName}}
	}
	return spans
}

func TestAnalyzedExtractor(t *testing.T) {
	config := make(map[string]map[string]float64)
	config["serviceA"] = make(map[string]float64)
	config["serviceA"]["opA"] = 0

	config["serviceB"] = make(map[string]float64)
	config["serviceB"]["opB"] = 0.5

	config["serviceC"] = make(map[string]float64)
	config["serviceC"]["opC"] = 1

	tests := []extractorTestCase{
		{"No priority - No service match", createTestSpans("serviceZ", "opA"), 0, RateNone},
		{"No priority - No name match", createTestSpans("serviceA", "opZ"), 0, RateNone},
		{"No priority - Match - 0", createTestSpans("serviceA", "opA"), 0, 0},
		{"No priority - Match - 0.5", createTestSpans("serviceB", "opB"), 0, 0.5},
		{"No priority - Match - 1", createTestSpans("serviceC", "opC"), 0, 1},
		{"Priority 1 - No service match", createTestSpans("serviceZ", "opA"), 1, RateNone},
		{"Priority 1 - No name match", createTestSpans("serviceA", "opZ"), 1, RateNone},
		{"Priority 1 - Match - 0", createTestSpans("serviceA", "opA"), 1, 0},
		{"Priority 1 - Match - 0.5", createTestSpans("serviceB", "opB"), 1, 0.5},
		{"Priority 1 - Match - 1", createTestSpans("serviceC", "opC"), 1, 1},
		{"Priority 2 - No service match", createTestSpans("serviceZ", "opA"), 2, RateNone},
		{"Priority 2 - No name match", createTestSpans("serviceA", "opZ"), 2, RateNone},
		{"Priority 2 - Match - 0", createTestSpans("serviceA", "opA"), 2, 0},
		{"Priority 2 - Match - 0.5", createTestSpans("serviceB", "opB"), 2, 1},
		{"Priority 2 - Match - 1", createTestSpans("serviceC", "opC"), 2, 1},
	}

	for _, test := range tests {
		testExtractor(t, NewFixedRateExtractor(config), test)
	}
}

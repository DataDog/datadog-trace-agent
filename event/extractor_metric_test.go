package event

import (
	"math/rand"
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
)

func createTestSpansWithEventRate(eventRate float64) []*model.WeightedSpan {
	spans := make([]*model.WeightedSpan, 1000)
	for i, _ := range spans {
		spans[i] = &model.WeightedSpan{Span: &model.Span{TraceID: rand.Uint64(), Service: "test", Name: "test"}}
		if eventRate >= 0 {
			spans[i].SetMetric(model.KeySamplingRateEventExtraction, eventRate)
		}
	}
	return spans
}

func TestMetricBasedExtractor(t *testing.T) {
	tests := []extractorTestCase{
		{"No priority - Missing extraction rate", createTestSpansWithEventRate(-1), 0, RateNone},
		{"No priority - Extraction rate = 0", createTestSpansWithEventRate(0), 0, 0},
		{"No priority - Extraction rate = 0.5", createTestSpansWithEventRate(0.5), 0, 0.5},
		{"No priority - Extraction rate = 1", createTestSpansWithEventRate(1), 0, 1},
		{"Priority 1 - Missing extraction rate", createTestSpansWithEventRate(-1), 1, RateNone},
		{"Priority 1 - Extraction rate = 0", createTestSpansWithEventRate(0), 1, 0},
		{"Priority 1 - Extraction rate = 0.5", createTestSpansWithEventRate(0.5), 1, 0.5},
		{"Priority 1 - Extraction rate = 1", createTestSpansWithEventRate(1), 1, 1},
		// Priority 2 should have extraction rate of 1 so long as any extraction rate is set and > 0
		{"Priority 2 - Missing extraction rate", createTestSpansWithEventRate(-1), 2, RateNone},
		{"Priority 2 - Extraction rate = 0", createTestSpansWithEventRate(0), 2, 0},
		{"Priority 2 - Extraction rate = 0.5", createTestSpansWithEventRate(0.5), 2, 1},
		{"Priority 2 - Extraction rate = 1", createTestSpansWithEventRate(1), 2, 1},
	}

	for _, test := range tests {
		testExtractor(t, NewMetricBasedExtractor(), test)
	}
}

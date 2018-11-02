package event

import (
	"math/rand"
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
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
	tests := []struct {
		name                   string
		spans                  []*model.WeightedSpan
		priority               int
		expectedExtractionRate float64
	}{
		{"No priority - Missing extraction rate", createTestSpansWithEventRate(-1), 0, UnknownRate},
		{"No priority - Extraction rate = 0", createTestSpansWithEventRate(0), 0, 0},
		{"No priority - Extraction rate = 0.5", createTestSpansWithEventRate(0.5), 0, 0.5},
		{"No priority - Extraction rate = 1", createTestSpansWithEventRate(1), 0, 1},
		{"Priority 1 - Missing extraction rate", createTestSpansWithEventRate(-1), 1, UnknownRate},
		{"Priority 1 - Extraction rate = 0", createTestSpansWithEventRate(0), 1, 0},
		{"Priority 1 - Extraction rate = 0.5", createTestSpansWithEventRate(0.5), 1, 0.5},
		{"Priority 1 - Extraction rate = 1", createTestSpansWithEventRate(1), 1, 1},
		// Priority 2 should have extraction rate of 1 so long as any extraction rate is set and > 0
		{"Priority 2 - Missing extraction rate", createTestSpansWithEventRate(-1), 2, UnknownRate},
		{"Priority 2 - Extraction rate = 0", createTestSpansWithEventRate(0), 2, 0},
		{"Priority 2 - Extraction rate = 0.5", createTestSpansWithEventRate(0.5), 2, 1},
		{"Priority 2 - Extraction rate = 1", createTestSpansWithEventRate(1), 2, 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			ae := NewMetricBasedExtractor()

			total := 0
			extracted := 0

			for _, span := range test.spans {
				extract, rate := ae.Extract(span, test.priority)

				total++

				if extract {
					extracted++
				}

				assert.EqualValues(test.expectedExtractionRate, rate)
			}

			if test.expectedExtractionRate != UnknownRate {
				// Assert extraction rate with 10% delta
				assert.InDelta(test.expectedExtractionRate, float64(extracted)/float64(total), test.expectedExtractionRate*0.1)
			}
		})
	}
}

package event

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

type extractorTestCase struct {
	name                   string
	spans                  []*model.WeightedSpan
	priority               model.SamplingPriority
	expectedExtractionRate float64
}

func testExtractor(t *testing.T, extractor Extractor, testCase extractorTestCase) {
	t.Run(testCase.name, func(t *testing.T) {
		assert := assert.New(t)

		total := 0
		extracted := 0

		for _, span := range testCase.spans {
			extract, rate := extractor.Extract(span, testCase.priority)

			total++

			if extract {
				extracted++
			}

			assert.EqualValues(testCase.expectedExtractionRate, rate)
		}

		if testCase.expectedExtractionRate != RateNone {
			// Assert extraction rate with 10% delta
			assert.InDelta(testCase.expectedExtractionRate, float64(extracted)/float64(total), testCase.expectedExtractionRate*0.1)
		}
	})
}

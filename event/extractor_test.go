package event

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/agent"
	"github.com/stretchr/testify/assert"
)

type extractorTestCase struct {
	name                   string
	spans                  []*agent.WeightedSpan
	priority               agent.SamplingPriority
	expectedExtractionRate float64
}

func testExtractor(t *testing.T, extractor Extractor, testCase extractorTestCase) {
	t.Run(testCase.name, func(t *testing.T) {
		assert := assert.New(t)

		total := 0

		for _, span := range testCase.spans {
			event, rate, ok := extractor.Extract(span, testCase.priority)

			total++

			if ok {
				assert.EqualValues(span.Span, event.Span)
				assert.EqualValues(testCase.priority, event.Priority)
			} else {
				rate = -1
			}

			assert.EqualValues(testCase.expectedExtractionRate, rate)
		}
	})
}

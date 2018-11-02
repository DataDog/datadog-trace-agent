package event

import (
	"math/rand"
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestProcessor(t *testing.T) {
	tests := []struct {
		name                 string
		extractorRates       []float64
		samplerRates         []float64
		expectedExtractedPct float64
		expectedSampledPct   float64
		deltaPct             float64
	}{
		{"No extractors", nil, nil, 0, 0, 0},

		// Test Extractors
		{"Extractor(0) - No samplers", []float64{0.}, nil, 0, 0, 0},
		{"Extractor(0.5) - No samplers", []float64{0.5}, nil, 0.5, 1, 0.1},
		{"Extractor(1) - No samplers", []float64{1}, nil, 1, 1, 0},
		{"Extractor(-1, 0.8) - No samplers", []float64{UnknownRate, 0.8}, nil, 0.8, 1, 0.1},
		{"Extractor(-1, -1, 0.8) - No samplers", []float64{UnknownRate, UnknownRate, 0.8}, nil, 0.8, 1, 0.1},

		// Test Samplers
		{"Extractor(1) - Sampler(0)", []float64{1}, []float64{0}, 1, 0, 0},
		{"Extractor(1) - Sampler(0.5)", []float64{1}, []float64{0.5}, 1, 0.5, 0.1},
		{"Extractor(1) - Sampler(1)", []float64{1}, []float64{1}, 1, 1, 0},
		{"Extractor(1) - Sampler(0.5, 0.5)", []float64{1}, []float64{0.5, 0.5}, 1, 0.5 * 0.5, 0.1},
		{"Extractor(1) - Sampler(-1, 0.5)", []float64{1}, []float64{UnknownRate, 0.5}, 1, 0.5, 0.1},

		// Test Extractor and Sampler combinations
		{"Extractor(-1, 0.8) - Sampler(0.8, 0.5)", []float64{-1, 0.8}, []float64{0.8, 0.5}, 0.8, 0.8 * 0.5, 0.1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			extractors := make([]Extractor, len(test.extractorRates))
			for i, rate := range test.extractorRates {
				extractors[i] = &MockExtractor{rate}
			}

			samplers := make([]Sampler, len(test.samplerRates))
			for i, rate := range test.samplerRates {
				samplers[i] = &MockSampler{rate}
			}

			p := NewProcessor(extractors, samplers)

			testSpans := createTestSpans("test", "test")
			testTrace := model.ProcessedTrace{WeightedTrace: testSpans}

			events, extracted := p.Process(testTrace)
			total := len(testSpans)
			returned := len(events)

			expectedExtracted := float64(total) * test.expectedExtractedPct
			assert.InDelta(expectedExtracted, extracted, expectedExtracted*test.deltaPct)

			expectedReturned := expectedExtracted * test.expectedSampledPct
			assert.InDelta(expectedReturned, returned, expectedReturned*test.deltaPct)
		})
	}
}

type MockExtractor struct {
	rate float64
}

func (e *MockExtractor) Extract(s *model.WeightedSpan, priority int) (bool, float64) {
	if e.rate >= 0 {
		return rand.Float64() < e.rate, e.rate
	}

	return false, UnknownRate
}

type MockSampler struct {
	rate float64
}

func (e *MockSampler) Start() {}
func (e *MockSampler) Stop()  {}

func (e *MockSampler) Sample(event *model.APMEvent) (bool, float64) {
	if e.rate >= 0 {
		return rand.Float64() < e.rate, e.rate
	}

	return false, UnknownRate
}

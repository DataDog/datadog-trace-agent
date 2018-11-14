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
		samplerRate          float64
		expectedExtractedPct float64
		expectedSampledPct   float64
		deltaPct             float64
	}{
		{"No extractors", nil, -1, 0, 0, 0},

		// Test Extractors
		{"Extractor(0) - Sampler(1)", []float64{0}, 1, 0, 0, 0},
		{"Extractor(0.5) - Sampler(1)", []float64{0.5}, 1, 0.5, 1, 0.1},
		{"Extractor(-1, 0.8) - Sampler(1)", []float64{-1, 0.8}, 1, 0.8, 1, 0.1},
		{"Extractor(-1, -1, 0.8) - Sampler(1)", []float64{-1, -1, 0.8}, 1, 0.8, 1, 0.1},

		// Test Sampler
		{"Extractor(1) - Sampler(0)", []float64{1}, 0, 1, 0, 0},
		{"Extractor(1) - Sampler(0.5)", []float64{1}, 0.5, 1, 0.5, 0.1},
		{"Extractor(1) - Sampler(1)", []float64{1}, 1, 1, 1, 0},

		// Test Extractor and Sampler combinations
		{"Extractor(-1, 0.8) - Sampler(0.8)", []float64{-1, 0.8}, 0.8, 0.8, 0.8, 0.1},
	}

	testClientSampleRate := 0.3
	testPreSampleRate := 0.5

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			extractors := make([]Extractor, len(test.extractorRates))
			for i, rate := range test.extractorRates {
				extractors[i] = &MockExtractor{Rate: rate}
			}

			sampler := &MockSampler{Rate: test.samplerRate}
			p := NewProcessor(extractors, sampler)

			testSpans := createTestSpans("test", "test")
			testTrace := model.ProcessedTrace{WeightedTrace: testSpans}
			testTrace.Root = testSpans[0].Span
			testTrace.Root.SetPreSampleRate(testPreSampleRate)
			testTrace.Root.SetClientTraceSampleRate(testClientSampleRate)

			p.Start()
			events, extracted := p.Process(testTrace)
			p.Stop()
			total := len(testSpans)
			returned := len(events)

			expectedExtracted := float64(total) * test.expectedExtractedPct
			assert.InDelta(expectedExtracted, extracted, expectedExtracted*test.deltaPct)

			expectedReturned := expectedExtracted * test.expectedSampledPct
			assert.InDelta(expectedReturned, returned, expectedReturned*test.deltaPct)

			assert.EqualValues(1, sampler.StartCalls)
			assert.EqualValues(extracted, sampler.SampleCalls)
			assert.EqualValues(1, sampler.StopCalls)

			for _, event := range events {
				assert.EqualValues(test.expectedExtractedPct, event.GetExtractionSampleRate())
				assert.EqualValues(testClientSampleRate, event.GetClientTraceSampleRate())
				assert.EqualValues(testPreSampleRate, event.GetPreSampleRate())
			}
		})
	}
}

type MockExtractor struct {
	Rate float64
}

func (e *MockExtractor) Extract(s *model.WeightedSpan, priority model.SamplingPriority) (bool, float64, bool) {
	if e.Rate >= 0 {
		return rand.Float64() < e.Rate, e.Rate, true
	}

	return false, 0, false
}

type MockSampler struct {
	Rate float64

	StartCalls  int
	StopCalls   int
	SampleCalls int
}

func (s *MockSampler) Start() {
	s.StartCalls++
}

func (s *MockSampler) Stop() {
	s.StopCalls++
}

func (s *MockSampler) Sample(event *model.APMEvent) (bool, float64) {
	s.SampleCalls++

	return rand.Float64() < s.Rate, s.Rate
}

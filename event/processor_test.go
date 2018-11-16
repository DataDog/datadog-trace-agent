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
		priority             model.SamplingPriority
		expectedExtractedPct float64
		expectedSampledPct   float64
		deltaPct             float64
	}{
		// Name: <extraction rates>/<maxEPSSampler rate>/<priority>
		{"none/1/none", nil, 1, model.PriorityNone, 0, 0, 0},

		// Test Extractors
		{"0/1/none", []float64{0}, 1, model.PriorityNone, 0, 0, 0},
		{"0.5/1/none", []float64{0.5}, 1, model.PriorityNone, 0.5, 1, 0.1},
		{"-1,0.8/1/none", []float64{-1, 0.8}, 1, model.PriorityNone, 0.8, 1, 0.1},
		{"-1,-1,-0.8/1/none", []float64{-1, -1, 0.8}, 1, model.PriorityNone, 0.8, 1, 0.1},

		// Test MaxEPS sampler
		{"1/0/none", []float64{1}, 0, model.PriorityNone, 1, 0, 0},
		{"1/0.5/none", []float64{1}, 0.5, model.PriorityNone, 1, 0.5, 0.1},
		{"1/1/none", []float64{1}, 1, model.PriorityNone, 1, 1, 0},

		// Test Extractor and Sampler combinations
		{"-1,0.8/0.8/none", []float64{-1, 0.8}, 0.8, model.PriorityNone, 0.8, 0.8, 0.1},
		{"-1,0.8/0.8/autokeep", []float64{-1, 0.8}, 0.8, model.PriorityAutoKeep, 0.8, 0.8, 0.1},
		// Test userkeep bypass of max eps
		{"-1,0.8/0.8/userkeep", []float64{-1, 0.8}, 0.8, model.PriorityUserKeep, 0.8, 1, 0.1},
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

			sampler := &MockEventSampler{Rate: test.samplerRate}
			p := newProcessor(extractors, sampler)

			testSpans := createTestSpans("test", "test")
			testTrace := model.ProcessedTrace{WeightedTrace: testSpans}
			testTrace.Root = testSpans[0].Span
			testTrace.Root.SetPreSampleRate(testPreSampleRate)
			testTrace.Root.SetClientTraceSampleRate(testClientSampleRate)
			if test.priority != model.PriorityNone {
				testTrace.Root.SetSamplingPriority(test.priority)
			}

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
			assert.EqualValues(1, sampler.StopCalls)

			expectedSampleCalls := extracted
			if test.priority == model.PriorityUserKeep {
				expectedSampleCalls = 0
			}
			assert.EqualValues(expectedSampleCalls, sampler.SampleCalls)

			for _, event := range events {
				assert.EqualValues(test.expectedExtractedPct, event.GetExtractionSampleRate())
				assert.EqualValues(test.expectedSampledPct, event.GetMaxEPSSampleRate())
				assert.EqualValues(testClientSampleRate, event.GetClientTraceSampleRate())
				assert.EqualValues(testPreSampleRate, event.GetPreSampleRate())
			}
		})
	}
}

type MockExtractor struct {
	Rate float64
}

func (e *MockExtractor) Extract(s *model.WeightedSpan, priority model.SamplingPriority) (*model.Event, float64, bool) {
	if e.Rate < 0 {
		return nil, 0, false
	}

	return &model.Event{
		Span:     s.Span,
		Priority: priority,
	}, e.Rate, true
}

type MockEventSampler struct {
	Rate float64

	StartCalls  int
	StopCalls   int
	SampleCalls int
}

func (s *MockEventSampler) Start() {
	s.StartCalls++
}

func (s *MockEventSampler) Stop() {
	s.StopCalls++
}

func (s *MockEventSampler) Sample(event *model.Event) (bool, float64) {
	s.SampleCalls++

	return rand.Float64() < s.Rate, s.Rate
}

package event

import (
	"math/rand"
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func createTraces(serviceName string, operationName string, topLevel bool, hasPriority bool, priority int) []model.ProcessedTrace {
	traces := make([]model.ProcessedTrace, 0, 2000)

	for i := 0; i < 1000; i++ {
		ws := model.WeightedSpan{TopLevel: topLevel, Span: &model.Span{TraceID: rand.Uint64(), Service: serviceName, Name: operationName}}
		if hasPriority {
			ws.SetSamplingPriority(priority)
		}
		wt := model.WeightedTrace{&ws}
		trace := model.ProcessedTrace{WeightedTrace: wt, Root: ws.Span}
		trace.Sampled = rand.Int()%2 == 0
		traces = append(traces, trace)
	}

	return traces
}

func TestAnalyzedExtractor(t *testing.T) {
	config := make(map[string]map[string]float64)
	config["myService"] = make(map[string]float64)
	config["myService"]["myOperation"] = 0.5

	config["mySampledService"] = make(map[string]float64)
	config["mySampledService"]["myOperation"] = 0

	tests := []struct {
		name                   string
		traces                 []model.ProcessedTrace
		expectedExtractionRate float64
	}{
		{"Top-level service and span name match", createTraces("myService", "myOperation", true, false, 0), 0.5},
		{"Top-level service name doesn't match", createTraces("otherService", "myOperation", true, false, 0), 0},
		{"Top-level span name doesn't match", createTraces("myService", "otherOperation", true, false, 0), 0},
		{"Top-level service and span name don't match", createTraces("otherService", "otherOperation", true, false, 0), 0},
		{"Non top-level service and span name match", createTraces("myService", "myOperation", false, false, 0), 0.5},
		{"Non top-level service name doesn't match", createTraces("otherService", "myOperation", false, false, 0), 0},
		{"Non top-level span name doesn't match", createTraces("myService", "otherOperation", false, false, 0), 0},
		{"Non top-level service and span name don't match", createTraces("otherService", "otherOperation", false, false, 0), 0},
		{"Match, sampling rate 0, no priority", createTraces("mySampledService", "myOperation", true, false, 0), 0},
		{"Match, sampling rate 0, priority 1", createTraces("mySampledService", "myOperation", true, true, 1), 0},
		{"Match, sampling rate 0, priority 2", createTraces("mySampledService", "myOperation", true, true, 2), 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			ae := NewFixedRateExtractor(config)

			sampled := 0

			for _, trace := range test.traces {
				extractedEvents := ae.Extract(trace)

				for _, event := range extractedEvents {
					assert.EqualValues(test.expectedExtractionRate, event.GetExtractionSampleRate())
					sampled++
				}
			}

			// Assert extraction rate with 10% delta
			assert.InDelta(test.expectedExtractionRate, float64(sampled)/float64(len(test.traces)), test.expectedExtractionRate*0.1)
		})
	}
}

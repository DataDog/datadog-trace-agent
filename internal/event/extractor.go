package event

import (
	"github.com/DataDog/datadog-trace-agent/internal/agent"
)

// Extractor extracts APM events from matching spans.
type Extractor interface {
	// Extract decides whether to extract an APM event from the provided span with the specified priority and returns
	// the extracted event along with a suggested extraction sample rate and a true value. If no trace was extracted
	// the bool value will be false and the other values should not be used.
	Extract(span *agent.WeightedSpan, priority agent.SamplingPriority) (event *agent.Event, rate float64, ok bool)
}

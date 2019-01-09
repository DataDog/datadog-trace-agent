package event

import (
	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/DataDog/datadog-trace-agent/internal/sampler"
)

// Extractor extracts APM events from matching spans.
type Extractor interface {
	// Extract decides whether to extract an APM event from the provided span with the specified priority and returns
	// a suggested extraction sample rate and a bool value. If no event was extracted the bool value will be false and
	// the rate should not be used.
	Extract(span *agent.WeightedSpan, priority sampler.SamplingPriority) (rate float64, ok bool)
}

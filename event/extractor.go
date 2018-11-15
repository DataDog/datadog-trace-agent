package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// Extractor extracts APM events from matching spans.
type Extractor interface {
	// Extract decides whether to extract an APM event from the provided span with the specified priority and returns
	// the extracted event along with a suggested extraction sample rate (or nil if no event was extracted).
	Extract(span *model.WeightedSpan, priority model.SamplingPriority) (*model.Event, float64)
}

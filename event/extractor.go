package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// Extractor is a special kind of sampler that decides whether an APM event should be extracted from a span.
type Extractor interface {
	// Extract decides whether to extract an APM event from the provided span with the specified priority, returning
	// true if an extraction should happen or false otherwise. It also returns a rate specifying the extraction rate
	// taken into account for this decision. If the returned rate is RateNone, then this extractor
	// didn't find anything to extract.
	Extract(span *model.WeightedSpan, priority model.SamplingPriority) (extract bool, rate float64)
}

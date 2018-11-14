package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// Extractor is a special kind of sampler that decides whether an APM event should be extracted from a span.
type Extractor interface {
	// Extract decides whether to extract an APM event from the provided span with the specified priority.
	//
	// If the extractor made an extraction decision on this span it returns true/false depending on whether an event
	// should be extracted from the provided span or not. It also returns a rate specifying the extraction rate taken
	// into account for this decision. Finally, it returns a third value of true, specifying that a decision was made.
	//
	// If the extractor did not know what to do with the span (e.g. lacks any rules matching said span), then it
	// returns false as the third value and both `extract` and `rate` are not to be taken into account.
	Extract(span *model.WeightedSpan, priority model.SamplingPriority) (extract bool, rate float64, decided bool)
}

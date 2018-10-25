package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// Extractor extracts APM event spans from a trace.
type Extractor interface {
	// Extract extracts APM event spans from the given weighted trace information and returns them.
	Extract(t model.ProcessedTrace) []*model.APMEvent
}

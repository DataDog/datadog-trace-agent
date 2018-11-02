package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// Sampler samples APM events according to implementation-defined techniques.
type Sampler interface {
	// Start tells this sampler to bootstrap whatever it needs to answer `Sample` requests.
	Start()
	// Sample decides whether to sample the provided event or not and also returns the applied sampling rate.
	Sample(event *model.APMEvent) (sampled bool, rate float64)
	// Stop tells this sampler to stop anything that was bootstrapped in `Start`.
	Stop()
}

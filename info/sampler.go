package info

import "stackstate-trace-agent/sampler"

// SamplerInfo represents internal stats and state of a sampler
type SamplerInfo struct {
	// EngineType contains the type of the engine (tells old sampler and new distributed sampler apart)
	EngineType string
	// Stats contains statistics about what the sampler is doing.
	Stats SamplerStats
	// State is the internal state of the sampler (for debugging mostly)
	State sampler.InternalState
}

// SamplerStats contains sampler statistics
type SamplerStats struct {
	// KeptTPS is the number of traces kept (average per second for last flush)
	KeptTPS float64
	// TotalTPS is the total number of traces (average per second for last flush)
	TotalTPS float64
}

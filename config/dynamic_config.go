package config

// DynamicConfig contains configuration items which may change
// dynamically over time.
type DynamicConfig struct {
	// RateByService contains the rate for each service/env tuple,
	// used in priority sampling by client libs.
	RateByService RateByService
}

// NewDynamicConfig creates a new dynamic config object.
func NewDynamicConfig() *DynamicConfig {
	// Not much logic here now, as RateByService is fine with
	// being used unintialized, but external packages should use this.
	return &DynamicConfig{}
}

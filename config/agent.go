package config

import (
	"strconv"
	"time"

	log "github.com/cihub/seelog"
)

// AgentConfig handles the interpretation of the configuration (with default
// behaviors) is one place. It is also a simple structure to share across all
// the Agent components, with 100% safe and reliable values.
type AgentConfig struct {
	APIEndpoint string
	APIKey      string
	APIEnabled  bool

	BucketInterval   time.Duration // the size of our pre-aggregation per bucket
	OldestSpanCutoff int64         // maximum time we wait before discarding straggling spans, in ns

	ExtraAggregators []string

	SamplerQuantiles []float64
}

// NewDefaultAgentConfig returns a configuration with the default values
func NewDefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		APIEndpoint: "http://localhost:8012/api/v0.1",
		APIKey:      "",
		APIEnabled:  true,

		BucketInterval:   time.Duration(5) * time.Second,
		OldestSpanCutoff: time.Duration(5 * time.Second).Nanoseconds(),

		ExtraAggregators: []string{},

		SamplerQuantiles: []float64{0, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 1},
	}
}

// NewAgentConfig creates the AgentConfig from the standard config. It handles all the cases.
func NewAgentConfig(conf *File) (*AgentConfig, error) {
	c := NewDefaultAgentConfig()

	if v, e := conf.Get("trace.api", "endpoint"); e == nil {
		c.APIEndpoint = v
	}

	if v, e := conf.Get("trace.api", "api_key"); e == nil {
		c.APIKey = v
	} else {
		return c, e
	}

	c.APIEnabled = conf.GetBool("trace.api", "enabled", true)

	if v, e := conf.GetInt("trace.concentrator", "bucket_size_seconds"); e == nil {
		c.BucketInterval = time.Duration(v) * time.Second
	}

	if v, e := conf.GetStrArray("trace.concentrator", "extra_aggregators", ","); e == nil {
		c.ExtraAggregators = v
	} else {
		log.Info("No aggregator configuration, using defaults")
	}

	if v, e := conf.GetStrArray("trace.concentrator", "quantiles", ","); e == nil {
		quantiles := make([]float64, len(v))
		for index, q := range v {
			value, err := strconv.ParseFloat(q, 64)
			if err != nil {
				return nil, err
			}
			quantiles[index] = value
		}
		c.SamplerQuantiles = quantiles
	}

	return c, nil
}

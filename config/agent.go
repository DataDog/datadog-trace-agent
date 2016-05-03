package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	log "github.com/cihub/seelog"
)

// AgentConfig handles the interpretation of the configuration (with default
// behaviors) is one place. It is also a simple structure to share across all
// the Agent components, with 100% safe and reliable values.
type AgentConfig struct {
	// Global
	HostName string

	// API
	APIEndpoint string
	APIKey      string
	APIEnabled  bool

	// Concentrator
	BucketInterval   time.Duration // the size of our pre-aggregation per bucket
	OldestSpanCutoff int64         // maximum time we wait before discarding straggling spans, in ns
	ExtraAggregators []string

	// Sampler
	SamplerQuantiles []float64

	// Grapher
	Topology       bool // enable topology graph collection
	TracePortsList []string
}

// NewDefaultAgentConfig returns a configuration with the default values
func NewDefaultAgentConfig() *AgentConfig {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}

	return &AgentConfig{
		HostName: hostname,

		APIEndpoint: "http://localhost:8012/api/v0.1",
		APIKey:      "",
		APIEnabled:  true,

		BucketInterval:   time.Duration(5) * time.Second,
		OldestSpanCutoff: time.Duration(30 * time.Second).Nanoseconds(),
		ExtraAggregators: []string{},

		SamplerQuantiles: []float64{0, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 1},

		Topology:       false,
		TracePortsList: []string{},
	}
}

// NewAgentConfig creates the AgentConfig from the standard config. It handles all the cases.
func NewAgentConfig(conf *File) (*AgentConfig, error) {
	c := NewDefaultAgentConfig()

	if v, e := conf.Get("trace.config", "hostname"); e == nil {
		c.HostName = v
	}
	if c.HostName == "" {
		return c, errors.New("no hostname defined")
	}

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

	if v, e := conf.GetInt("trace.concentrator", "oldest_span_cutoff_seconds"); e == nil {
		c.OldestSpanCutoff = (time.Duration(v) * time.Second).Nanoseconds()
	}

	if v, e := conf.GetStrArray("trace.concentrator", "extra_aggregators", ","); e == nil {
		c.ExtraAggregators = v
	} else {
		log.Debug("No aggregator configuration, using defaults")
	}

	if v, e := conf.GetStrArray("trace.sampler", "quantiles", ","); e == nil {
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

	if tracePortsList, e := conf.GetStrArray("trace.grapher", "port_whitelist", ","); e == nil {
		log.Debugf("Tracing ports : %s", tracePortsList)
		c.TracePortsList = tracePortsList
	}

	c.Topology = conf.GetBool("trace.grapher", "enabled", false)

	return c, nil
}

package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/ini.v1"

	log "github.com/cihub/seelog"
)

// AgentConfig handles the interpretation of the configuration (with default
// behaviors) is one place. It is also a simple structure to share across all
// the Agent components, with 100% safe and reliable values.
type AgentConfig struct {
	// Global
	HostName string

	// API
	APIEndpoint    string
	APIKey         string
	APIEnabled     bool
	APIFlushTraces bool

	// Concentrator
	BucketInterval    time.Duration // the size of our pre-aggregation per bucket
	OldestSpanCutoff  int64         // maximum time we wait before discarding straggling spans, in ns
	ExtraAggregators  []string
	LatencyResolution time.Duration

	// Sampler configuration
	// Quantile sampler
	SamplerQuantiles []float64
	// Signature sampler
	SamplerTheta  float64
	SamplerJitter float64
	SamplerSMin   float64

	// Grapher
	Topology       bool // enable topology graph collection
	TracePortsList []string

	// Receiver
	ConnectionLimit int // for rate-limiting, how many unique connections to allow in a lease period (30s)

	// internal telemetry
	StatsdHost string
	StatsdPort int
}

// mergeConfig applies overrides from the dd-agent config to the
// trace agent
func mergeConfig(c *AgentConfig, f *ini.File) {
	m, err := f.GetSection("Main")
	if err != nil {
		return
	}

	if v := m.Key("hostname").MustString(""); v != "" {
		c.HostName = v
	} else {
		log.Info("Failed to parse hostname from dd-agent config")
	}

	if v := m.Key("api_key").MustString(""); v != "" {
		c.APIKey = v
	} else {
		log.Info("Failed to parse api_key from dd-agent config")
	}

	if v := m.Key("bind_host").MustString(""); v != "" {
		c.StatsdHost = v
	}

	if v := m.Key("dogstatsd_port").MustInt(-1); v != -1 {
		c.StatsdPort = v
	}
}

// NewDefaultAgentConfig returns a configuration with the default values
func NewDefaultAgentConfig() *AgentConfig {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	ac := &AgentConfig{
		HostName:       hostname,
		APIEndpoint:    "https://trace.datadoghq.com/api/v0.1",
		APIKey:         "",
		APIEnabled:     true,
		APIFlushTraces: true,

		BucketInterval:    time.Duration(10) * time.Second,
		OldestSpanCutoff:  time.Duration(60 * time.Second).Nanoseconds(),
		ExtraAggregators:  []string{},
		LatencyResolution: time.Millisecond,

		SamplerQuantiles: []float64{0.10, 0.50, 0.90, 1},

		SamplerSMin:   1,
		SamplerTheta:  60, // 1 min
		SamplerJitter: 0.2,

		Topology:       false,
		TracePortsList: []string{},

		ConnectionLimit: 2000,

		StatsdHost: "localhost",
		StatsdPort: 8125,
	}

	// Check the classic agent's config for overrides
	if dd, _ := ini.Load("/etc/dd-agent/datadog.conf"); dd != nil {
		log.Debug("Found dd-agent config file, applying overrides")
		mergeConfig(ac, dd)
	}

	return ac
}

// NewAgentConfig creates the AgentConfig from the standard config. It handles all the cases.
func NewAgentConfig(conf *File) (*AgentConfig, error) {
	c := NewDefaultAgentConfig()

	// Allow overrides of previously set config without erroring
	if v, _ := conf.Get("trace.config", "hostname"); v != "" {
		c.HostName = v
	}

	if v, _ := conf.Get("trace.api", "api_key"); v != "" {
		c.APIKey = v
	}

	if v, _ := conf.Get("trace.api", "endpoint"); v != "" {
		c.APIEndpoint = v
	}

	if v, e := conf.Get("trace.api", "flush_traces"); e == nil && v == "false" {
		c.APIFlushTraces = false
	}

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

	if v, e := conf.Get("trace.concentrator", "latency_res"); e == nil {
		switch v {
		case "millisecond":
			c.LatencyResolution = time.Millisecond
		case "microsecond":
			c.LatencyResolution = time.Microsecond
		case "nanosecond":
			c.LatencyResolution = time.Nanosecond
		}
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

	if v, e := conf.GetInt("trace.sampler", "score_threshold"); e == nil {
		c.SamplerSMin = float64(v)
	}
	if v, e := conf.GetInt("trace.sampler", "trace_period"); e == nil {
		c.SamplerTheta = float64(v)
	}
	if v, e := conf.GetInt("trace.sampler", "score_jitter"); e == nil {
		c.SamplerJitter = float64(v)
	}

	if tracePortsList, e := conf.GetStrArray("trace.grapher", "port_whitelist", ","); e == nil {
		log.Debugf("Tracing ports : %s", tracePortsList)
		c.TracePortsList = tracePortsList
	}

	if v, e := conf.GetInt("trace.receiver", "connection_limit"); e == nil {
		c.ConnectionLimit = v
	}

	return c, nil
}

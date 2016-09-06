package config

import (
	"errors"
	"os"
	"strings"
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
	APIEndpoints []string
	APIKeys      []string
	APIEnabled   bool

	// Concentrator
	BucketInterval   time.Duration // the size of our pre-aggregation per bucket
	OldestSpanCutoff int64         // maximum time we wait before discarding straggling spans, in ns
	ExtraAggregators []string

	// Sampler configuration
	ScoreThreshold  float64
	SignaturePeriod time.Duration
	ScoreJitter     float64
	TPSMax          float64

	// Receiver
	ReceiverPort    int
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

	if v := m.Key("api_key").Strings(","); len(v) != 0 {
		c.APIKeys = v
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
		HostName:     hostname,
		APIEndpoints: []string{"https://trace.agent.datadoghq.com/api/v0.1"},
		APIKeys:      []string{""},
		APIEnabled:   true,

		BucketInterval:   time.Duration(10) * time.Second,
		OldestSpanCutoff: time.Duration(60 * time.Second).Nanoseconds(),
		ExtraAggregators: []string{},

		ScoreThreshold:  5,
		SignaturePeriod: time.Minute,
		ScoreJitter:     0.1,
		TPSMax:          100,

		ReceiverPort:    7777,
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
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		c.APIKeys = vals
	}

	if v, _ := conf.Get("trace.api", "endpoint"); v != "" {
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		c.APIEndpoints = vals
	}

	if len(c.APIKeys) != len(c.APIEndpoints) {
		return c, errors.New("every API key needs to have an explicit endpoint associated")
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

	if v, e := conf.GetFloat("trace.sampler", "score_threshold"); e == nil {
		c.ScoreThreshold = v
	}
	if v, e := conf.GetFloat("trace.sampler", "trace_period"); e == nil {
		c.SignaturePeriod = time.Duration(int(v * 1e9))
	}
	if v, e := conf.GetFloat("trace.sampler", "score_jitter"); e == nil {
		c.ScoreJitter = v
	}
	if v, e := conf.GetFloat("trace.sampler", "max_tps"); e == nil {
		c.TPSMax = v
	}

	if v, e := conf.GetInt("trace.receiver", "receiver_port"); e == nil {
		c.ReceiverPort = v
	}

	if v, e := conf.GetInt("trace.receiver", "connection_limit"); e == nil {
		c.ConnectionLimit = v
	}

	return c, nil
}

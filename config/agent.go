package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
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
	APIEndpoints []string
	APIKeys      []string
	APIEnabled   bool

	// Concentrator
	BucketInterval time.Duration // the size of our pre-aggregation per bucket
	// OldestSpanCutoff is the maximum time we wait before discarding straggling spans, in ns.
	// A span is considered too old when its end time is before now, minus this value.
	OldestSpanCutoff int64
	ExtraAggregators []string

	// Sampler configuration
	ExtraSampleRate float64
	MaxTPS          float64

	// Receiver
	ReceiverHost    string
	ReceiverPort    int
	ConnectionLimit int // for rate-limiting, how many unique connections to allow in a lease period (30s)

	// internal telemetry
	StatsdHost string
	StatsdPort int
}

// mergeEnv applies overrides from environment variables to the trace agent configuration
func mergeEnv(c *AgentConfig) {
	if v := os.Getenv("DD_HOSTNAME"); v != "" {
		c.HostName = v
	}

	if v := os.Getenv("DD_API_KEY"); v != "" {
		log.Info("overriding API key from env API_KEY value")
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		c.APIKeys = vals
	}

	if v := os.Getenv("DD_RECEIVER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			log.Info("Failed to parse DD_RECEIVER_PORT: it should be a port number")
		} else {
			c.ReceiverPort = port
		}
	}

	if v := os.Getenv("DD_BIND_HOST"); v != "" {
		c.StatsdHost = v
		c.ReceiverHost = v
	}

	if v := os.Getenv("DD_DOGSTATSD_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			log.Info("Failed to parse DD_DOGSTATSD_PORT: it should be a port number")
		} else {
			c.StatsdPort = port
		}
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
		APIEndpoints: []string{"https://trace.agent.datadoghq.com"},
		APIKeys:      []string{},
		APIEnabled:   true,

		BucketInterval:   time.Duration(10) * time.Second,
		OldestSpanCutoff: time.Duration(time.Minute).Nanoseconds(),
		ExtraAggregators: []string{},

		ExtraSampleRate: 1.0,
		MaxTPS:          10,

		ReceiverHost:    "localhost",
		ReceiverPort:    7777,
		ConnectionLimit: 2000,

		StatsdHost: "localhost",
		StatsdPort: 8125,
	}

	return ac
}

// NewAgentConfig creates the AgentConfig from the standard config
func NewAgentConfig(conf *File) (*AgentConfig, error) {
	c := NewDefaultAgentConfig()

	// Inherit all relevant config from dd-agent
	m, err := conf.GetSection("Main")
	if err == nil {
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
			c.ReceiverHost = v
		}

		if v := m.Key("dogstatsd_port").MustInt(-1); v != -1 {
			c.StatsdPort = v
		}
	}

	// When available inherit APM specific config
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

	if v, e := conf.GetFloat("trace.sampler", "extra_sample_rate"); e == nil {
		c.ExtraSampleRate = v
	}
	if v, e := conf.GetFloat("trace.sampler", "max_traces_per_second"); e == nil {
		c.MaxTPS = v
	}

	if v, e := conf.GetInt("trace.receiver", "receiver_port"); e == nil {
		c.ReceiverPort = v
	}

	if v, e := conf.GetInt("trace.receiver", "connection_limit"); e == nil {
		c.ConnectionLimit = v
	}

	// environment variables have precedence among defaults and the config file
	mergeEnv(c)
	return c, nil
}

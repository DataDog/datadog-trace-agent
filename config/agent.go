package config

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"

	log "github.com/cihub/seelog"
	"github.com/go-ini/ini"
)

// AgentConfig handles the interpretation of the configuration (with default
// behaviors) in one place. It is also a simple structure to share across all
// the Agent components, with 100% safe and reliable values.
// It is exposed with expvar, so make sure to exclude any sensible field
// from JSON encoding.
type AgentConfig struct {
	Enabled bool

	// Global
	HostName   string
	DefaultEnv string // the traces will default to this environment

	// API
	APIEndpoints            []string
	APIKeys                 []string `json:"-"` // never publish this
	APIEnabled              bool
	APIPayloadBufferMaxSize int

	// Concentrator
	BucketInterval   time.Duration // the size of our pre-aggregation per bucket
	ExtraAggregators []string

	// Sampler configuration
	ExtraSampleRate float64
	PreSampleRate   float64
	MaxTPS          float64

	// Receiver
	ReceiverHost    string
	ReceiverPort    int
	ConnectionLimit int // for rate-limiting, how many unique connections to allow in a lease period (30s)
	ReceiverTimeout int

	// internal telemetry
	StatsdHost string
	StatsdPort int

	// logging
	LogLevel    string
	LogFilePath string

	// watchdog
	MaxMemory        float64       // MaxMemory is the threshold (bytes allocated) above which program panics and exits, to be restarted
	MaxConnections   int           // MaxConnections is the threshold (opened TCP connections) above which program panics and exits, to be restarted
	WatchdogInterval time.Duration // WatchdogInterval is the delay between 2 watchdog checks

	// http/s proxying
	Proxy *ProxySettings
}

// mergeEnv applies overrides from environment variables to the trace agent configuration
func mergeEnv(c *AgentConfig) {
	if v := os.Getenv("DD_APM_ENABLED"); v == "true" {
		c.Enabled = true
	} else if v == "false" {
		c.Enabled = false
	}

	if v := os.Getenv("DD_HOSTNAME"); v != "" {
		log.Info("overriding hostname from env DD_HOSTNAME value")
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

	if v := os.Getenv("DD_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
}

// getHostname shells out to obtain the hostname used by the infra agent
// falling back to os.Hostname() if it is unavailable
func getHostname() (string, error) {
	ddAgentPy := "/opt/datadog-agent/embedded/bin/python"
	getHostnameCmd := "from utils.hostname import get_hostname; print get_hostname()"

	cmd := exec.Command(ddAgentPy, "-c", getHostnameCmd)
	cmd.Env = []string{"PYTHONPATH=/opt/datadog-agent/agent"}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Infof("error retrieving dd-agent hostname, falling back to os.Hostname(): %v", err)
		return os.Hostname()
	}

	hostname := strings.TrimSpace(stdout.String())

	if hostname == "" {
		log.Infof("error retrieving dd-agent hostname, falling back to os.Hostname(): %s", stderr.String())
		return os.Hostname()
	}

	return hostname, err
}

// NewDefaultAgentConfig returns a configuration with the default values
func NewDefaultAgentConfig() *AgentConfig {
	hostname, err := getHostname()
	if err != nil {
		hostname = ""
	}
	ac := &AgentConfig{
		HostName:                hostname,
		DefaultEnv:              "none",
		APIEndpoints:            []string{"https://trace.agent.datadoghq.com"},
		APIKeys:                 []string{},
		APIEnabled:              true,
		APIPayloadBufferMaxSize: 16 * 1024 * 1024,

		BucketInterval:   time.Duration(10) * time.Second,
		ExtraAggregators: []string{},

		ExtraSampleRate: 1.0,
		PreSampleRate:   1.0,
		MaxTPS:          10,

		ReceiverHost:    "localhost",
		ReceiverPort:    8126,
		ConnectionLimit: 2000,

		StatsdHost: "localhost",
		StatsdPort: 8125,

		LogLevel:    "INFO",
		LogFilePath: "/var/log/datadog/trace-agent.log",

		MaxMemory:        1e9,
		MaxConnections:   5000,
		WatchdogInterval: time.Minute,
	}

	return ac
}

// NewAgentConfig creates the AgentConfig from the standard config
func NewAgentConfig(conf *File, legacyConf *File) (*AgentConfig, error) {
	c := NewDefaultAgentConfig()
	var m *ini.Section
	var err error

	if conf == nil {
		goto APM_CONF
	}

	// Inherit all relevant config from dd-agent
	m, err = conf.GetSection("Main")
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

		// non_local_traffic is a shorthand in dd-agent configuration that is
		// equivalent to setting `bind_host: 0.0.0.0`. Respect this flag
		// since it defaults to true in Docker and saves us a command-line param
		if v := strings.ToLower(m.Key("non_local_traffic").MustString("")); v == "yes" || v == "true" {
			c.StatsdHost = "0.0.0.0"
			c.ReceiverHost = "0.0.0.0"
		}

		if v := m.Key("dogstatsd_port").MustInt(-1); v != -1 {
			c.StatsdPort = v
		}
		if v := m.Key("log_level").MustString(""); v != "" {
			c.LogLevel = v
		}

		if p := getProxySettings(m); p.Host != "" {
			c.Proxy = p
		}
	}

APM_CONF:
	// When available inherit APM specific config
	if legacyConf != nil {
		// try to use the legacy config file passed via `-configfile`
		conf = legacyConf
	}

	if conf == nil {
		goto ENV_CONF
	}

	if v, _ := conf.Get("Main", "apm_enabled"); v == "true" {
		c.Enabled = true
	}

	if v, _ := conf.Get("trace.config", "env"); v != "" {
		c.DefaultEnv = model.NormalizeTag(v)
	}

	if v, _ := conf.Get("trace.config", "log_level"); v != "" {
		c.LogLevel = v
	}

	if v, _ := conf.Get("trace.config", "log_file"); v != "" {
		c.LogFilePath = v
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

	if v, e := conf.GetInt("trace.api", "payload_buffer_max_size"); e == nil {
		c.APIPayloadBufferMaxSize = v
	}

	if v, e := conf.GetInt("trace.concentrator", "bucket_size_seconds"); e == nil {
		c.BucketInterval = time.Duration(v) * time.Second
	}

	if v, e := conf.GetStrArray("trace.concentrator", "extra_aggregators", ","); e == nil {
		c.ExtraAggregators = v
	} else {
		log.Debug("No aggregator configuration, using defaults")
	}

	if v, e := conf.GetFloat("trace.sampler", "extra_sample_rate"); e == nil {
		c.ExtraSampleRate = v
	}
	if v, e := conf.GetFloat("trace.sampler", "pre_sample_rate"); e == nil {
		c.PreSampleRate = v
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

	if v, e := conf.GetInt("trace.receiver", "timeout"); e == nil {
		c.ReceiverTimeout = v
	}

	if v, e := conf.GetFloat("trace.watchdog", "max_memory"); e == nil {
		c.MaxMemory = v
	}

	if v, e := conf.GetInt("trace.watchdog", "max_connections"); e == nil {
		c.MaxConnections = v
	}

	if v, e := conf.GetInt("trace.watchdog", "check_delay_seconds"); e == nil {
		c.WatchdogInterval = time.Duration(v) * time.Second
	}

ENV_CONF:
	// environment variables have precedence among defaults and the config file
	mergeEnv(c)

	// check for api-endpoint parity after all possible overrides have been applied
	if len(c.APIKeys) == 0 {
		return c, errors.New("you must specify an API Key, either via a configuration file or the DD_API_KEY env var")
	}

	if len(c.APIKeys) != len(c.APIEndpoints) {
		return c, errors.New("every API key needs to have an explicit endpoint associated")
	}
	return c, nil
}

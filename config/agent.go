package config

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
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
	APIEndpoint string
	APIKey      string `json:"-"` // never publish this
	APIEnabled  bool

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

	// Writers
	ServiceWriterConfig writerconfig.ServiceWriterConfig
	StatsWriterConfig   writerconfig.StatsWriterConfig
	TraceWriterConfig   writerconfig.TraceWriterConfig

	// internal telemetry
	StatsdHost string
	StatsdPort int

	// logging
	LogLevel             string
	LogFilePath          string
	LogThrottlingEnabled bool

	// watchdog
	MaxMemory        float64       // MaxMemory is the threshold (bytes allocated) above which program panics and exits, to be restarted
	MaxCPU           float64       // MaxCPU is the max UserAvg CPU the program should consume
	MaxConnections   int           // MaxConnections is the threshold (opened TCP connections) above which program panics and exits, to be restarted
	WatchdogInterval time.Duration // WatchdogInterval is the delay between 2 watchdog checks

	// http/s proxying
	ProxyURL          *url.URL
	SkipSSLValidation bool

	// filtering
	Ignore map[string][]string

	// ReplaceTags is used to filter out sensitive information from tag values.
	// It maps tag keys to a set of replacements.
	// TODO(x): Introduce into Agent5 ini config. Currently only supported in 6.
	ReplaceTags []*ReplaceRule

	// transaction analytics
	AnalyzedRateByService map[string]float64

	// infrastructure agent binary
	DDAgentBin string // DDAgentBin will be "" for Agent5 scenarios
}

// NewDefaultAgentConfig returns a configuration with the default values
func NewDefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		Enabled:     true,
		DefaultEnv:  "none",
		APIEndpoint: "https://trace.agent.datadoghq.com",
		APIKey:      "",
		APIEnabled:  true,

		BucketInterval:   time.Duration(10) * time.Second,
		ExtraAggregators: []string{"http.status_code"},

		ExtraSampleRate: 1.0,
		PreSampleRate:   1.0,
		MaxTPS:          10,

		ReceiverHost:    "localhost",
		ReceiverPort:    8126,
		ConnectionLimit: 2000,

		ServiceWriterConfig: writerconfig.DefaultServiceWriterConfig(),
		StatsWriterConfig:   writerconfig.DefaultStatsWriterConfig(),
		TraceWriterConfig:   writerconfig.DefaultTraceWriterConfig(),

		StatsdHost: "localhost",
		StatsdPort: 8125,

		LogLevel:             "INFO",
		LogFilePath:          DefaultLogFilePath,
		LogThrottlingEnabled: true,

		MaxMemory:        5e8, // 500 Mb, should rarely go above 50 Mb
		MaxCPU:           0.5, // 50%, well behaving agents keep below 5%
		MaxConnections:   200, // in practice, rarely goes over 20
		WatchdogInterval: time.Minute,

		Ignore:                make(map[string][]string),
		AnalyzedRateByService: make(map[string]float64),
	}
}

// getHostname shells out to obtain the hostname used by the infra agent
// falling back to os.Hostname() if it is unavailable
func getHostname(ddAgentBin string) (string, error) {
	var cmd *exec.Cmd

	// In Agent 6 we will have an Agent binary defined.
	if ddAgentBin != "" {
		cmd = exec.Command(ddAgentBin, "hostname")
		cmd.Env = []string{}
	} else {
		getHostnameCmd := "from utils.hostname import get_hostname; print get_hostname()"
		cmd = exec.Command(defaultDDAgentPy, "-c", getHostnameCmd)
		cmd.Env = []string{defaultDDAgentPyEnv}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Copying all environment variables to child process
	// Windows: Required, so the child process can load DLLs, etc.
	// Linux:   Optional, but will make use of DD_HOSTNAME and DOCKER_DD_AGENT if they exist
	osEnv := os.Environ()
	cmd.Env = append(osEnv, cmd.Env...)

	err := cmd.Run()
	if err != nil {
		return os.Hostname()
	}

	hostname := strings.TrimSpace(stdout.String())

	if hostname == "" {
		return os.Hostname()
	}

	return hostname, nil

}

// NewAgentConfig creates the AgentConfig from the standard config
func NewAgentConfig(conf *File, legacyConf *File, agentYaml *YamlAgentConfig) (*AgentConfig, error) {
	c := NewDefaultAgentConfig()
	var err error

	if conf != nil {
		// Agent 5
		err = mergeIniConfig(c, conf)
		if err != nil {
			return nil, err
		}
	}

	if agentYaml != nil {
		// Agent 6
		err = mergeYamlConfig(c, agentYaml)
		if err != nil {
			return nil, err
		}
	}

	if legacyConf != nil {
		err = mergeIniConfig(c, legacyConf)
		if err != nil {
			return nil, err
		}
	}

	// environment variables have precedence among defaults and the config file
	mergeEnv(c)

	// check for api-endpoint parity after all possible overrides have been applied
	if c.APIKey == "" {
		return c, errors.New("you must specify an API Key, either via a configuration file or the DD_API_KEY env var")
	}

	// If hostname isn't provided in the configuration, try to guess it.
	if c.HostName == "" {
		hostname, err := getHostname(c.DDAgentBin)
		if err != nil {
			return c, errors.New("failed to automatically set the hostname, you must specify it via configuration for or the DD_HOSTNAME env var")
		}
		c.HostName = hostname
	}

	return c, nil
}

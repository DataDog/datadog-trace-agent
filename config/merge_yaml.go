package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"regexp"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/DataDog/datadog-trace-agent/backoff"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/osutil"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
	log "github.com/cihub/seelog"
)

// YamlAgentConfig is a structure used for marshaling the datadog.yaml configuration
// available in Agent versions >= 6
type YamlAgentConfig struct {
	APIKey            string `yaml:"api_key"`
	HostName          string `yaml:"hostname"`
	LogLevel          string `yaml:"log_level"`
	Proxy             proxy  `yaml:"proxy"`
	SkipSSLValidation *bool  `yaml:"skip_ssl_validation"`

	StatsdPort int `yaml:"dogstatsd_port"`

	TraceAgent traceAgent `yaml:"apm_config"`
}

type proxy struct {
	HTTP    string   `yaml:"http"`
	HTTPS   string   `yaml:"https"`
	NoProxy []string `yaml:"no_proxy"`
}

type traceAgent struct {
	Enabled            *bool          `yaml:"enabled"`
	Endpoint           string         `yaml:"apm_dd_url"`
	Env                string         `yaml:"env"`
	ExtraSampleRate    float64        `yaml:"extra_sample_rate"`
	MaxTracesPerSecond float64        `yaml:"max_traces_per_second"`
	IgnoreResources    []string       `yaml:"ignore_resources"`
	LogFilePath        string         `yaml:"log_file"`
	ReplaceTags        []*ReplaceRule `yaml:"replace_tags"`
	ReceiverPort       int            `yaml:"receiver_port"`
	ConnectionLimit    int            `yaml:"connection_limit"`
	APMNonLocalTraffic *bool          `yaml:"apm_non_local_traffic"`

	WatchdogMaxMemory float64 `yaml:"max_memory"`
	WatchdogMaxCPUPct float64 `yaml:"max_cpu_percent"`
	WatchdogMaxConns  int     `yaml:"max_connections"`

	TraceWriter   traceWriter   `yaml:"trace_writer"`
	ServiceWriter serviceWriter `yaml:"service_writer"`
	StatsWriter   statsWriter   `yaml:"stats_writer"`

	AnalyzedRateByServiceLegacy map[string]float64 `yaml:"analyzed_rate_by_service"`
	AnalyzedSpans               map[string]float64 `yaml:"analyzed_spans"`

	DDAgentBin string `yaml:"dd_agent_bin"`
}

type ReplaceRule struct {
	Name    string         `yaml:"name"`
	Pattern string         `yaml:"pattern"`
	Re      *regexp.Regexp `yaml:"-"`
	Repl    string         `yaml:"repl"`
}

type traceWriter struct {
	MaxSpansPerPayload     int                    `yaml:"max_spans_per_payload"`
	FlushPeriod            int                    `yaml:"flush_period_seconds"`
	UpdateInfoPeriod       int                    `yaml:"update_info_period_seconds"`
	QueueablePayloadSender queueablePayloadSender `yaml:"queue"`
}

type serviceWriter struct {
	UpdateInfoPeriod       int                    `yaml:"update_info_period_seconds"`
	FlushPeriod            int                    `yaml:"flush_period_seconds"`
	QueueablePayloadSender queueablePayloadSender `yaml:"queue"`
}

type statsWriter struct {
	MaxEntriesPerPayload   int                    `yaml:"max_entries_per_payload"`
	UpdateInfoPeriod       int                    `yaml:"update_info_period_seconds"`
	QueueablePayloadSender queueablePayloadSender `yaml:"queue"`
}

type queueablePayloadSender struct {
	MaxAge            int   `yaml:"max_age_seconds"`
	MaxQueuedBytes    int64 `yaml:"max_bytes"`
	MaxQueuedPayloads int   `yaml:"max_payloads"`
	BackoffDuration   int   `yaml:"exp_backoff_max_duration_seconds"`
	BackoffBase       int   `yaml:"exp_backoff_base_milliseconds"`
	BackoffGrowth     int   `yaml:"exp_backoff_growth_base"`
}

// newYamlFromBytes returns a new YamlAgentConfig for the provided byte array.
func newYamlFromBytes(bytes []byte) (*YamlAgentConfig, error) {
	var yamlConf YamlAgentConfig

	if err := yaml.Unmarshal(bytes, &yamlConf); err != nil {
		return nil, fmt.Errorf("failed to parse yaml configuration: %s", err)
	}
	return &yamlConf, nil
}

// NewYamlIfExists returns a new YamlAgentConfig if the given configPath is exists.
func NewYaml(configPath string) (*YamlAgentConfig, error) {
	fileContent, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	return newYamlFromBytes(fileContent)
}

func (c *AgentConfig) loadYamlConfig(yc *YamlAgentConfig) {
	if yc == nil {
		return
	}

	if yc.APIKey != "" {
		c.APIKey = yc.APIKey
	}
	if yc.HostName != "" {
		c.Hostname = yc.HostName
	}
	if yc.LogLevel != "" {
		c.LogLevel = yc.LogLevel
	}
	if yc.StatsdPort > 0 {
		c.StatsdPort = yc.StatsdPort
	}

	// respect Agent proxy configuration knowing we only have to support the APIEndpoint HTTPS case
	if yc.Proxy.HTTPS != "" {
		traceAgentNoProxy := false
		for _, host := range yc.Proxy.NoProxy {
			if host == c.APIEndpoint {
				log.Info("Trace Agent endpoint matches proxy.no_proxy list item '%s': not using any proxy", host)
				traceAgentNoProxy = true
				break
			}
		}

		if !traceAgentNoProxy {
			url, err := url.Parse(yc.Proxy.HTTPS)
			if err == nil {
				c.ProxyURL = url
			} else {
				log.Errorf("Failed to parse proxy URL from proxy.https configuration: %s", err)
			}
		}
	}

	if yc.SkipSSLValidation != nil {
		c.SkipSSLValidation = *yc.SkipSSLValidation
	}

	if yc.TraceAgent.Enabled != nil {
		c.Enabled = *yc.TraceAgent.Enabled
	}

	if yc.TraceAgent.Endpoint != "" {
		c.APIEndpoint = yc.TraceAgent.Endpoint
	}

	if yc.TraceAgent.LogFilePath != "" {
		c.LogFilePath = yc.TraceAgent.LogFilePath
	}

	if yc.TraceAgent.Env != "" {
		c.DefaultEnv = model.NormalizeTag(yc.TraceAgent.Env)
	}

	if yc.TraceAgent.ReceiverPort > 0 {
		c.ReceiverPort = yc.TraceAgent.ReceiverPort
	}

	if yc.TraceAgent.ConnectionLimit > 0 {
		c.ConnectionLimit = yc.TraceAgent.ConnectionLimit
	}

	if yc.TraceAgent.ExtraSampleRate > 0 {
		c.ExtraSampleRate = yc.TraceAgent.ExtraSampleRate
	}
	if yc.TraceAgent.MaxTracesPerSecond > 0 {
		c.MaxTPS = yc.TraceAgent.MaxTracesPerSecond
	}

	if len(yc.TraceAgent.IgnoreResources) > 0 {
		c.Ignore["resource"] = yc.TraceAgent.IgnoreResources
	}

	if rt := yc.TraceAgent.ReplaceTags; rt != nil {
		err := compileReplaceRules(rt)
		if err != nil {
			osutil.Exitf("replace_tags: %s", err)
		}
		c.ReplaceTags = rt
	}

	if yc.TraceAgent.APMNonLocalTraffic != nil && *yc.TraceAgent.APMNonLocalTraffic {
		c.ReceiverHost = "0.0.0.0"
	}

	// undocumented
	if yc.TraceAgent.WatchdogMaxCPUPct > 0 {
		c.MaxCPU = yc.TraceAgent.WatchdogMaxCPUPct / 100
	}
	if yc.TraceAgent.WatchdogMaxMemory > 0 {
		c.MaxMemory = yc.TraceAgent.WatchdogMaxMemory
	}
	if yc.TraceAgent.WatchdogMaxConns > 0 {
		c.MaxConnections = yc.TraceAgent.WatchdogMaxConns
	}

	// undocumented
	c.ServiceWriterConfig = readServiceWriterConfigYaml(yc.TraceAgent.ServiceWriter)
	c.StatsWriterConfig = readStatsWriterConfigYaml(yc.TraceAgent.StatsWriter)
	c.TraceWriterConfig = readTraceWriterConfigYaml(yc.TraceAgent.TraceWriter)

	// undocumented deprecated
	c.AnalyzedRateByServiceLegacy = yc.TraceAgent.AnalyzedRateByServiceLegacy
	if len(yc.TraceAgent.AnalyzedRateByServiceLegacy) > 0 {
		log.Warn("analyzed_rate_by_service is deprecated, please use analyzed_spans instead")
	}
	// undocumeted
	for key, rate := range yc.TraceAgent.AnalyzedSpans {
		serviceName, operationName, err := parseServiceAndOp(key)
		if err != nil {
			log.Errorf("Error when parsing names", err)
			continue
		}

		if _, ok := c.AnalyzedSpansByService[serviceName]; !ok {
			c.AnalyzedSpansByService[serviceName] = make(map[string]float64)
		}
		c.AnalyzedSpansByService[serviceName][operationName] = rate
	}

	// undocumented
	c.DDAgentBin = defaultDDAgentBin
	if yc.TraceAgent.DDAgentBin != "" {
		c.DDAgentBin = yc.TraceAgent.DDAgentBin
	}
}

func readServiceWriterConfigYaml(yc serviceWriter) writerconfig.ServiceWriterConfig {
	c := writerconfig.DefaultServiceWriterConfig()

	if yc.FlushPeriod > 0 {
		c.FlushPeriod = getDuration(yc.FlushPeriod)
	}

	if yc.UpdateInfoPeriod > 0 {
		c.UpdateInfoPeriod = getDuration(yc.UpdateInfoPeriod)
	}

	c.SenderConfig = readQueueablePayloadSenderConfigYaml(yc.QueueablePayloadSender)
	return c
}

func readStatsWriterConfigYaml(yc statsWriter) writerconfig.StatsWriterConfig {
	c := writerconfig.DefaultStatsWriterConfig()

	if yc.MaxEntriesPerPayload > 0 {
		c.MaxEntriesPerPayload = yc.MaxEntriesPerPayload
	}

	if yc.UpdateInfoPeriod > 0 {
		c.UpdateInfoPeriod = getDuration(yc.UpdateInfoPeriod)
	}

	c.SenderConfig = readQueueablePayloadSenderConfigYaml(yc.QueueablePayloadSender)

	return c
}

func readTraceWriterConfigYaml(yc traceWriter) writerconfig.TraceWriterConfig {
	c := writerconfig.DefaultTraceWriterConfig()

	if yc.MaxSpansPerPayload > 0 {
		c.MaxSpansPerPayload = yc.MaxSpansPerPayload
	}
	if yc.FlushPeriod > 0 {
		c.FlushPeriod = getDuration(yc.FlushPeriod)
	}
	if yc.UpdateInfoPeriod > 0 {
		c.UpdateInfoPeriod = getDuration(yc.UpdateInfoPeriod)
	}

	c.SenderConfig = readQueueablePayloadSenderConfigYaml(yc.QueueablePayloadSender)

	return c
}

func readQueueablePayloadSenderConfigYaml(yc queueablePayloadSender) writerconfig.QueuablePayloadSenderConf {
	c := writerconfig.DefaultQueuablePayloadSenderConf()

	if yc.MaxAge != 0 {
		c.MaxAge = getDuration(yc.MaxAge)
	}

	if yc.MaxQueuedBytes != 0 {
		c.MaxQueuedBytes = yc.MaxQueuedBytes
	}

	if yc.MaxQueuedPayloads != 0 {
		c.MaxQueuedPayloads = yc.MaxQueuedPayloads
	}

	c.ExponentialBackoff = readExponentialBackoffConfigYaml(yc)

	return c
}

func readExponentialBackoffConfigYaml(yc queueablePayloadSender) backoff.ExponentialConfig {
	c := backoff.DefaultExponentialConfig()

	if yc.BackoffDuration > 0 {
		c.MaxDuration = getDuration(yc.BackoffDuration)
	}
	if yc.BackoffBase > 0 {
		c.Base = time.Duration(yc.BackoffBase) * time.Millisecond
	}
	if yc.BackoffGrowth > 0 {
		c.GrowthBase = yc.BackoffGrowth
	}

	return c
}

// compileReplaceRules compiles the regular expressions found in the replace rules.
// If it fails it returns the first error.
func compileReplaceRules(rules []*ReplaceRule) error {
	for _, r := range rules {
		if r.Name == "" {
			return errors.New(`all rules must have a "name" property (use "*" to target all)`)
		}
		if r.Pattern == "" {
			return errors.New(`all rules must have a "pattern"`)
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return fmt.Errorf("key %q: %s", r.Name, err)
		}
		r.Re = re
	}
	return nil
}

// getDuration returns the duration of the provided value in seconds
func getDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

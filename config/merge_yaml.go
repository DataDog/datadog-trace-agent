package config

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/DataDog/datadog-trace-agent/backoff"
	"github.com/DataDog/datadog-trace-agent/model"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/utils"
)

// YamlAgentConfig is a structure used for marshaling the datadog.yaml configuration
// available in Agent versions >= 6
type YamlAgentConfig struct {
	APIKey       string `yaml:"api_key"`
	HostName     string `yaml:"hostname"`
	LogLevel     string `yaml:"log_level"`
	Proxy        proxy  `yaml:"proxy"`
	ReceiverHost string ""

	TraceAgent traceAgent `yaml:"apm_config"`
}

type proxy struct {
	HTTP    string   `yaml:"http"`
	HTTPS   string   `yaml:"https"`
	NoProxy []string `yaml:"no_proxy"`
}

type traceAgent struct {
	Enabled            bool    `yaml:"enabled"`
	Env                string  `yaml:"env"`
	ExtraSampleRate    float64 `yaml:"extra_sample_rate"`
	MaxTracesPerSecond float64 `yaml:"max_traces_per_second"`
	Ignore             string  `yaml:"ignore_resource"`
	ReceiverPort       int     `yaml:"receiver_port"`
	ConnectionLimit    int     `yaml:"connection_limit"`
	NonLocalTraffic    string  `yaml:"trace_non_local_traffic"` // TODO: check that
	StatsdHost         string  `yaml:"apm_statsd_host"`         // TODO: check that
	StatsdPort         int     `yaml:"apm_statsd_port"`         // TODO: check that

	TraceWriter   traceWriter   `yaml:"trace_writer"`
	ServiceWriter serviceWriter `yaml:"service_writer"`
	StatsWriter   statsWriter   `yaml:"stats_writer"`

	AnalyzedRateByService map[string]float64 `yaml:"analyzed_rate_by_service"`
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
		return nil, fmt.Errorf("parse error: %s", err)
	}
	return &yamlConf, nil
}

// NewYamlIfExists returns a new YamlAgentConfig if the given configPath is exists.
func NewYamlIfExists(configPath string) (*YamlAgentConfig, error) {
	if utils.PathExists(configPath) {
		fileContent, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		return newYamlFromBytes(fileContent)
	}
	return nil, nil
}

func mergeYamlConfig(agentConf *AgentConfig, yc *YamlAgentConfig) error {
	if yc == nil {
		return nil
	}

	agentConf.APIKey = yc.APIKey
	agentConf.HostName = yc.HostName
	agentConf.Enabled = yc.TraceAgent.Enabled

	// respect Agent proxy configuration in the special case of the Trace Agent API
	if yc.Proxy.HTTPS != "" {
		traceAgentNoProxy := false
		for _, host := range yc.Proxy.NoProxy {
			if host == agentConf.APIEndpoint {
				log.Info("Trace Agent endpoint matches proxy.no_proxy list item '%s': not using any proxy", host)
				traceAgentNoProxy = true
				break
			}
		}

		if !traceAgentNoProxy {
			url, err := url.Parse(yc.Proxy.HTTPS)
			if err == nil {
				agentConf.ProxyURL = url
			} else {
				log.Errorf("Failed to parse proxy URL from proxy.https configuration: %s", err)
			}
		}
	}

	if yc.TraceAgent.ReceiverPort > 0 {
		agentConf.ReceiverPort = yc.TraceAgent.ReceiverPort
	}
	if yc.TraceAgent.StatsdPort > 0 {
		agentConf.StatsdPort = yc.TraceAgent.StatsdPort
	}
	if yc.TraceAgent.ExtraSampleRate > 0 {
		agentConf.ExtraSampleRate = yc.TraceAgent.ExtraSampleRate
	}
	if yc.TraceAgent.MaxTracesPerSecond > 0 {
		agentConf.MaxTPS = yc.TraceAgent.MaxTracesPerSecond
	}

	agentConf.Ignore["resource"] = strings.Split(yc.TraceAgent.Ignore, ",")

	if yc.TraceAgent.ConnectionLimit > 0 {
		agentConf.ConnectionLimit = yc.TraceAgent.ConnectionLimit
	}

	if yc.TraceAgent.Env != "" {
		agentConf.DefaultEnv = model.NormalizeTag(yc.TraceAgent.Env)
	}

	if yc.TraceAgent.StatsdHost != "" {
		yc.ReceiverHost = yc.TraceAgent.StatsdHost
	}

	// Respect non_local_traffic
	if v := strings.ToLower(yc.TraceAgent.NonLocalTraffic); v == "yes" || v == "true" {
		yc.TraceAgent.StatsdHost = "0.0.0.0"
		yc.ReceiverHost = "0.0.0.0"
	}

	agentConf.StatsdHost = yc.TraceAgent.StatsdHost
	agentConf.ReceiverHost = yc.ReceiverHost

	agentConf.ServiceWriterConfig = readServiceWriterConfigYaml(yc.TraceAgent.ServiceWriter)
	agentConf.StatsWriterConfig = readStatsWriterConfigYaml(yc.TraceAgent.StatsWriter)
	agentConf.TraceWriterConfig = readTraceWriterConfigYaml(yc.TraceAgent.TraceWriter)

	agentConf.AnalyzedRateByService = yc.TraceAgent.AnalyzedRateByService

	return nil
}

func readServiceWriterConfigYaml(yc serviceWriter) writerconfig.ServiceWriterConfig {
	c := writerconfig.DefaultServiceWriterConfig()

	if yc.FlushPeriod > 0 {
		c.FlushPeriod = utils.GetDuration(yc.FlushPeriod)
	}

	if yc.UpdateInfoPeriod > 0 {
		c.UpdateInfoPeriod = utils.GetDuration(yc.UpdateInfoPeriod)
	}

	c.SenderConfig = readQueueablePayloadSenderConfigYaml(yc.QueueablePayloadSender)
	return c
}

func readStatsWriterConfigYaml(yc statsWriter) writerconfig.StatsWriterConfig {
	c := writerconfig.DefaultStatsWriterConfig()

	if yc.UpdateInfoPeriod > 0 {
		c.UpdateInfoPeriod = utils.GetDuration(yc.UpdateInfoPeriod)
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
		c.FlushPeriod = utils.GetDuration(yc.FlushPeriod)
	}
	if yc.UpdateInfoPeriod > 0 {
		c.UpdateInfoPeriod = utils.GetDuration(yc.UpdateInfoPeriod)
	}

	c.SenderConfig = readQueueablePayloadSenderConfigYaml(yc.QueueablePayloadSender)

	return c
}

func readQueueablePayloadSenderConfigYaml(yc queueablePayloadSender) writerconfig.QueuablePayloadSenderConf {
	c := writerconfig.DefaultQueuablePayloadSenderConf()

	if yc.MaxAge != 0 {
		c.MaxAge = utils.GetDuration(yc.MaxAge)
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

// TODO: maybe this is too many options exposed?
func readExponentialBackoffConfigYaml(yc queueablePayloadSender) backoff.ExponentialConfig {
	c := backoff.DefaultExponentialConfig()

	if yc.BackoffDuration > 0 {
		c.MaxDuration = utils.GetDuration(yc.BackoffDuration)
	}
	if yc.BackoffBase > 0 {
		c.Base = time.Duration(yc.BackoffBase) * time.Millisecond
	}
	if yc.BackoffGrowth > 0 {
		c.GrowthBase = yc.BackoffGrowth
	}

	return c
}

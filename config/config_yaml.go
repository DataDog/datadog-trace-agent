package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strings"

	"github.com/DataDog/datadog-trace-agent/utils"
)

// YamlAgentConfig is a structure used for marshaling the datadog.yaml configuration
// available in Agent versions >= 6
type YamlAgentConfig struct {
	APIKey   string `yaml:"api_key"`
	HostName string `yaml:"hostname"`

	StatsdHost   string `yaml:"bind_host"`
	ReceiverHost string ""

	StatsdPort int    `yaml:"StatsdPort"`
	LogLevel   string `yaml:"log_level"`

	DefaultEnv string `yaml:"env"`

	TraceAgent struct {
		Enabled            bool    `yaml:"enabled"`
		Env                string  `yaml:"env"`
		ExtraSampleRate    float64 `yaml:"extra_sample_rate"`
		MaxTracesPerSecond float64 `yaml:"max_traces_per_second"`
		Ignore             string  `yaml:"ignore_resource"`
		ReceiverPort       int     `yaml:"receiver_port"`
		ConnectionLimit    int     `yaml:"connection_limit"`
		NonLocalTraffic    string  `yaml:"trace_non_local_traffic"`

		//TODO Merge these into config
		TraceWriter struct {
			MaxSpansPerPayload int   `yaml:"max_spans_per_payload"`
			FlushPeriod        int   `yaml:"flush_period_seconds"`
			UpdateInfoPeriod   int   `yaml:"update_info_period_seconds"`
			MaxAge             int   `yaml:"queue_max_age_seconds"`
			MaxQueuedBytes     int64 `yaml:"queue_max_bytes"`
			MaxQueuedPayloads  int   `yaml:"queue_max_payloads"`
			BackoffDuration    int   `yaml:"exp_backoff_max_duration_seconds"`
			BackoffBase        int   `yaml:"exp_backoff_base_milliseconds"`
			BackoffGrowth      int   `yaml:"exp_backoff_growth_base"`
		} `yaml:"trace_writer"`
		ServiceWriter struct {
			FlushPeriod       int   `yaml:"flush_period_seconds"`
			UpdateInfoPeriod  int   `yaml:"'update_info_period_seconds"`
			MaxAge            int   `yaml:"queue_max_age_seconds"`
			MaxQueuedBytes    int64 `yaml:"queue_max_bytes"`
			MaxQueuedPayloads int   `yaml:"queue_max_payloads"`
			BackoffDuration   int   `yaml:"exp_backoff_max_duration_seconds"`
			BackoffBase       int   `yaml:"exp_backoff_base_milliseconds"`
			BackoffGrowth     int   `yaml:"exp_backoff_growth_base"`
		} `yaml:"service_writer"`
		StatsWriter struct {
			UpdateInfoPeriod  int   `yaml:"update_info_period_seconds"`
			MaxAge            int   `yaml:"queue_max_age_seconds"`
			MaxQueuedBytes    int64 `yaml:"queue_max_bytes"`
			MaxQueuedPayloads int   `yaml:"queue_max_payloads"`
			BackoffDuration   int   `yaml:"exp_backoff_max_duration_seconds"`
			BackoffBase       int   `yaml:"exp_backoff_base_milliseconds"`
			BackoffGrowth     int   `yaml:"exp_backoff_growth_base"`
		} `yaml:"stats_writer"`
	} `yaml:"apm_config"`
}

// NewYamlIfExists returns a new YamlAgentConfig if the given configPath is exists.
func NewYamlIfExists(configPath string) (*YamlAgentConfig, error) {
	var yamlConf YamlAgentConfig
	if utils.PathExists(configPath) {
		fileContent, err := ioutil.ReadFile(configPath)
		if err = yaml.Unmarshal([]byte(fileContent), &yamlConf); err != nil {
			return nil, fmt.Errorf("parse error: %s", err)
		}
		return &yamlConf, nil
	}
	return nil, nil
}

func mergeYamlConfig(agentConf *AgentConfig, yc *YamlAgentConfig) (*AgentConfig, error) {
	agentConf.APIKey = yc.APIKey
	agentConf.HostName = yc.HostName
	agentConf.Enabled = yc.TraceAgent.Enabled
	agentConf.DefaultEnv = yc.DefaultEnv

	agentConf.ReceiverPort = yc.TraceAgent.ReceiverPort
	agentConf.ExtraSampleRate = yc.TraceAgent.ExtraSampleRate
	agentConf.MaxTPS = yc.TraceAgent.MaxTracesPerSecond

	agentConf.Ignore["resource"] = strings.Split(yc.TraceAgent.Ignore, ",")

	agentConf.ConnectionLimit = yc.TraceAgent.ConnectionLimit

	//Allow user to specify a different ENV for APM Specifically
	if yc.TraceAgent.Env != "" {
		agentConf.DefaultEnv = yc.TraceAgent.Env
	}

	if yc.StatsdHost != "" {
		yc.ReceiverHost = yc.StatsdHost
	}

	//Respect non_local_traffic
	if v := strings.ToLower(yc.TraceAgent.NonLocalTraffic); v == "yes" || v == "true" {
		yc.StatsdHost = "0.0.0.0"
		yc.ReceiverHost = "0.0.0.0"
	}

	agentConf.StatsdHost = yc.StatsdHost
	agentConf.ReceiverHost = yc.ReceiverHost

	//Trace Writer
	agentConf.TraceWriterConfig.FlushPeriod = utils.GetDuration(yc.TraceAgent.TraceWriter.FlushPeriod)
	agentConf.TraceWriterConfig.MaxSpansPerPayload = yc.TraceAgent.TraceWriter.MaxSpansPerPayload
	agentConf.TraceWriterConfig.UpdateInfoPeriod = utils.GetDuration(yc.TraceAgent.TraceWriter.UpdateInfoPeriod)
	agentConf.TraceWriterConfig.SenderConfig.MaxAge = utils.GetDuration(yc.TraceAgent.TraceWriter.MaxAge)
	agentConf.TraceWriterConfig.SenderConfig.MaxQueuedBytes = yc.TraceAgent.TraceWriter.MaxQueuedBytes
	agentConf.TraceWriterConfig.SenderConfig.MaxQueuedPayloads = yc.TraceAgent.TraceWriter.MaxQueuedPayloads
	agentConf.TraceWriterConfig.SenderConfig.ExponentialBackoff.MaxDuration = utils.GetDuration(yc.TraceAgent.TraceWriter.BackoffDuration)
	agentConf.TraceWriterConfig.SenderConfig.ExponentialBackoff.Base = utils.GetDuration(yc.TraceAgent.TraceWriter.BackoffBase)
	agentConf.TraceWriterConfig.SenderConfig.ExponentialBackoff.GrowthBase = yc.TraceAgent.TraceWriter.BackoffGrowth

	//Service Writer
	agentConf.ServiceWriterConfig.FlushPeriod = utils.GetDuration(yc.TraceAgent.ServiceWriter.FlushPeriod)
	agentConf.ServiceWriterConfig.UpdateInfoPeriod = utils.GetDuration(yc.TraceAgent.ServiceWriter.UpdateInfoPeriod)
	agentConf.ServiceWriterConfig.SenderConfig.MaxAge = utils.GetDuration(yc.TraceAgent.ServiceWriter.MaxAge)
	agentConf.ServiceWriterConfig.SenderConfig.MaxQueuedBytes = yc.TraceAgent.ServiceWriter.MaxQueuedBytes
	agentConf.ServiceWriterConfig.SenderConfig.MaxQueuedPayloads = yc.TraceAgent.ServiceWriter.MaxQueuedPayloads
	agentConf.ServiceWriterConfig.SenderConfig.ExponentialBackoff.MaxDuration = utils.GetDuration(yc.TraceAgent.ServiceWriter.BackoffDuration)
	agentConf.ServiceWriterConfig.SenderConfig.ExponentialBackoff.Base = utils.GetDuration(yc.TraceAgent.ServiceWriter.BackoffBase)
	agentConf.ServiceWriterConfig.SenderConfig.ExponentialBackoff.GrowthBase = yc.TraceAgent.ServiceWriter.BackoffGrowth

	//Stats Writer
	agentConf.StatsWriterConfig.UpdateInfoPeriod = utils.GetDuration(yc.TraceAgent.StatsWriter.UpdateInfoPeriod)
	agentConf.StatsWriterConfig.SenderConfig.MaxAge = utils.GetDuration(yc.TraceAgent.StatsWriter.MaxAge)
	agentConf.StatsWriterConfig.SenderConfig.MaxQueuedBytes = yc.TraceAgent.StatsWriter.MaxQueuedBytes
	agentConf.StatsWriterConfig.SenderConfig.MaxQueuedPayloads = yc.TraceAgent.StatsWriter.MaxQueuedPayloads
	agentConf.StatsWriterConfig.SenderConfig.ExponentialBackoff.MaxDuration = utils.GetDuration(yc.TraceAgent.StatsWriter.BackoffDuration)
	agentConf.StatsWriterConfig.SenderConfig.ExponentialBackoff.Base = utils.GetDuration(yc.TraceAgent.StatsWriter.BackoffBase)
	agentConf.StatsWriterConfig.SenderConfig.ExponentialBackoff.GrowthBase = yc.TraceAgent.StatsWriter.BackoffGrowth

	return agentConf, nil
}

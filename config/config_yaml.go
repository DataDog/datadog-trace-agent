package config

import (
	"bufio"
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"strings"
	"io/ioutil"

	"github.com/DataDog/datadog-trace-agent/utils"
)

// YamlAgentConfig is a structure used for marshaling the datadog.yaml configuration
// available in Agent versions >= 6
type YamlAgentConfig struct {
	APIKey          string `yaml:"api_key"`
	HostName        string `yaml:"hostname"`

	StatsdHost   string `yaml:"bind_host"`
	ReceiverHost string ""

	StatsdPort int    `yaml:"StatsdPort"`
	LogLevel   string `yaml:"log_level"`

	DefaultEnv string `yaml:"env"`

	TraceAgent struct {
		Enabled            bool              `yaml:"enabled"`
		Env                string              `yaml:"env"`
		ExtraSampleRate    float64             `yaml:"extra_sample_rate"`
		MaxTracesPerSecond float64             `yaml:"max_traces_per_second"`
		Ignore             string              `yaml:"ignore_resource"`
		ReceiverPort       int                 `yaml:"receiver_port"`
		ConnectionLimit    int                 `yaml:"connection_limit"`
		NonLocalTraffic    string              `yaml:"trace_non_local_traffic"`
	} `yaml:"trace_config"`
}

// ReadLines reads contents from a file and splits them by new lines.
func ReadLines(filename string) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var ret []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		ret = append(ret, scanner.Text())
	}
	return ret, scanner.Err()
}

// NewYamlIfExists returns a new YamlAgentConfig if the given configPath is exists.
func NewYamlIfExists(configPath string) (*YamlAgentConfig, error) {
	var yamlConf YamlAgentConfig
	if utils.PathExists(configPath) {
		// lines, err := ReadLines(configPath)
		// if err != nil {
		// 	return nil, fmt.Errorf("read error: %s", err)
		// }
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

	return agentConf, nil
}

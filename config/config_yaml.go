package config

import (
	"fmt"
  "os"
  "bufio"
	"strings"
	"gopkg.in/yaml.v2"
)

// YamlAgentConfig is a sturcutre used for marshaling the datadog.yaml configuratio
// available in Agent versions >= 6
type YamlAgentConfig struct {
	APIKey       string `yaml:"api_key"`
	Enabled      bool `yaml:"apm_enabled"`
	HostName     string `yaml:"hostname"`
	NonLocalTraffic string `yaml:"non_local_traffic"`

	StatsdHost    string  `yaml:"bind_host"`
  ReceiverHost  string  ""

	StatsdPort    int     `yaml:"StatsdPort"`
	LogLevel      string  `yaml:"log_level"`

	DefaultEnv    string `yaml:"env"`

  TraceAgent     struct {
		Env            string `yaml:"env"`
		ExtraSampleRate float64 `yaml:"extra_sample_rate"`
		MaxTracesPerSecond float64 `yaml:"max_traces_per_second"`
		TraceIgnore    struct {
			Ignore      map[string][]string `yaml:"resource"`
		}`yaml:"trace_ignore"`
		TraceReceiver   struct {
			ReceiverPort  int `yaml:"receiver_port"`
			ConnectionLimit int `yaml:"connection_limit"`
		}`yaml:"trace_receiver"`
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
	if PathExists(configPath) {
	  lines, err := ReadLines(configPath)
	  if err != nil {
	  	return nil, fmt.Errorf("read error: %s", err)
  	}
	  if err = yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &yamlConf); err != nil {
		  return nil, fmt.Errorf("parse error: %s", err)
	  }
	  return &yamlConf, nil
  }
return nil, nil
}

// PathExists returns a boolean indicating if the given path exists on the file system.
func PathExists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}

func mergeYamlConfig(agentConf *AgentConfig, yc *YamlAgentConfig) (*AgentConfig, error) {
	agentConf.APIKey = yc.APIKey
  agentConf.HostName = yc.HostName
	agentConf.Enabled = yc.Enabled
	agentConf.DefaultEnv = yc.DefaultEnv

	agentConf.ReceiverPort = yc.TraceAgent.TraceReceiver.ReceiverPort
	agentConf.ExtraSampleRate = yc.TraceAgent.ExtraSampleRate
  agentConf.MaxTPS = yc.TraceAgent.MaxTracesPerSecond
	agentConf.Ignore = yc.TraceAgent.TraceIgnore.Ignore
	agentConf.ConnectionLimit = yc.TraceAgent.TraceReceiver.ConnectionLimit


	//Allow user to specify a different ENV for APM Specifically
	if yc.TraceAgent.Env != "" {
		agentConf.DefaultEnv = yc.TraceAgent.Env
	}

	if yc.StatsdHost != "" {
		yc.ReceiverHost = yc.StatsdHost
	}

	//Respect non_local_traffic
	if v := strings.ToLower(yc.NonLocalTraffic); v == "yes" || v == "true" {
		yc.StatsdHost = "0.0.0.0"
		yc.ReceiverHost = "0.0.0.0"
	}
	
	agentConf.StatsdHost = yc.StatsdHost
	agentConf.ReceiverHost = yc.ReceiverHost

	return agentConf, nil
}

// SetupDDAgentConfig initializes the datadog-agent config with a YAML file.
// This is required for configuration to be available for container listeners.
func SetupDDAgentConfig(configPath string) error {
	// ddconfig.Datadog.AddConfigPath(configPath)
	// If they set a config file directly, let's try to honor that
	if strings.HasSuffix(configPath, ".yaml") {
		// ddconfig.Datadog.SetConfigFile(configPath)
	}

	// load the configuration
	// if err := ddconfig.Datadog.ReadInConfig(); err != nil {
	// 	return fmt.Errorf("unable to load Datadog config file: %s", err)
	// }

	return nil
}

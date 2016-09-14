package config

import (
	"strings"
	"time"

	"github.com/stretchr/testify/assert"

	"testing"

	"gopkg.in/ini.v1"
)

func TestGetStrArray(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[Main]\n\nports = 10,15,20,25"))
	conf := File{
		f,
		"some/path",
	}

	ports, err := conf.GetStrArray("Main", "ports", ",")
	assert.Nil(err)
	assert.Equal(ports, []string{"10", "15", "20", "25"})
}

func TestMergeConfig(t *testing.T) {
	assert := assert.New(t)
	agentConfig := AgentConfig{
		HostName: "hostname",

		APIEndpoints: []string{"http://localhost:8012/api/v0.1"},
		APIKeys:      []string{""},
		APIEnabled:   true,

		BucketInterval:   time.Duration(5) * time.Second,
		OldestSpanCutoff: time.Duration(30 * time.Second).Nanoseconds(),
		ExtraAggregators: []string{},

		StatsdHost: "localhost",
		StatsdPort: 8125,
	}

	ddAgentConf, _ := ini.Load([]byte("[Main]\n\nhostname=thing\napi_key=apikey_12"))
	mergeConfig(&agentConfig, ddAgentConf)
	assert.Equal("thing", agentConfig.HostName)
	assert.Equal([]string{"apikey_12"}, agentConfig.APIKeys)
}

func TestDDAgentMultiAPIKeys(t *testing.T) {
	assert := assert.New(t)
	agentConfig := NewDefaultAgentConfig()

	ddAgentConf, _ := ini.Load([]byte("[Main]\n\napi_key=foo, bar "))
	mergeConfig(agentConfig, ddAgentConf)
	assert.Equal([]string{"foo", "bar"}, agentConfig.APIKeys)
}
func TestConfigLoadAndMerge(t *testing.T) {
	assert := assert.New(t)

	defaultConfig := NewDefaultAgentConfig()

	configIni, _ := ini.Load([]byte(strings.Join([]string{
		"[trace.config]",
		"hostname = thing",
		"[trace.api]",
		"api_key = pommedapi",
		"[trace.concentrator]",
		"extra_aggregators=resource,error",
		"[trace.sampler]",
		"score_jitter=0.33",
	}, "\n")))

	configFile := &File{instance: configIni, Path: "whatever"}

	agentConfig, _ := NewAgentConfig(configFile)

	// Properly loaded attributes
	assert.Equal("thing", agentConfig.HostName)
	assert.Equal([]string{"pommedapi"}, agentConfig.APIKeys)
	assert.Equal([]string{"resource", "error"}, agentConfig.ExtraAggregators)
	assert.Equal(0.33, agentConfig.ScoreJitter)

	// Check some defaults
	assert.Equal(defaultConfig.BucketInterval, agentConfig.BucketInterval)
	assert.Equal(defaultConfig.OldestSpanCutoff, agentConfig.OldestSpanCutoff)
	assert.Equal(defaultConfig.TPSMax, agentConfig.TPSMax)
	assert.Equal(defaultConfig.StatsdHost, agentConfig.StatsdHost)
}

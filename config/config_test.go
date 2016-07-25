package config

import (
	"github.com/stretchr/testify/assert"
	"time"

	"gopkg.in/ini.v1"
	"testing"
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

		APIEndpoint: "http://localhost:8012/api/v0.1",
		APIKey:      "",
		APIEnabled:  true,

		BucketInterval:   time.Duration(5) * time.Second,
		OldestSpanCutoff: time.Duration(30 * time.Second).Nanoseconds(),
		ExtraAggregators: []string{},

		Topology:       false,
		TracePortsList: []string{},

		StatsdHost: "localhost",
		StatsdPort: 8125,
	}

	ddAgentConf, _ := ini.Load([]byte("[Main]\n\nhostname: thing\napi_key: apikey_2"))
	mergeConfig(&agentConfig, ddAgentConf)
	assert.Equal(agentConfig.HostName, "thing")
	assert.Equal(agentConfig.APIKey, "apikey_2")
}

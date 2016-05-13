package config

import (
	"github.com/stretchr/testify/assert"
	"time"

	"gopkg.in/ini.v1"
	"testing"
)

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

		SamplerQuantiles: []float64{0, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 1},

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

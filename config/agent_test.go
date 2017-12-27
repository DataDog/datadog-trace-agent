package config

import (
	"strings"

	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/go-ini/ini"
)

func TestPrioritySamplerConfig(t *testing.T) {
	assert := assert.New(t)
	agentConfig := NewDefaultAgentConfig()

	assert.True(agentConfig.PrioritySampling)
	assert.True(agentConfig.ScorePriority0Traces)

	dd, _ := ini.Load([]byte(strings.Join([]string{
		"[trace.sampler]",
	}, "\n")))

	conf := &File{instance: dd, Path: "whatever"}
	agentConfig, _ = NewAgentConfig(conf, nil)

	assert.True(agentConfig.PrioritySampling)
	assert.True(agentConfig.ScorePriority0Traces)

	dd, _ = ini.Load([]byte(strings.Join([]string{
		"[trace.sampler]",
		"priority_sampling = no",
		"score_priority_0_traces = no",
	}, "\n")))

	conf = &File{instance: dd, Path: "whatever"}
	agentConfig, _ = NewAgentConfig(conf, nil)

	assert.False(agentConfig.PrioritySampling)
	assert.False(agentConfig.ScorePriority0Traces)
}

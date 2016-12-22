package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
)

func BenchmarkAgentTraceProcessing(b *testing.B) {
	conf := config.NewDefaultAgentConfig()
	conf.APIKeys = append(conf.APIKeys, "")
	agent := NewAgent(conf)
	for i := 0; i < b.N; i++ {
		agent.Process(fixtures.RandomTrace())
	}
}

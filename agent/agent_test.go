package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
)

func BenchmarkAgentTraceProcessing(b *testing.B) {
	// Disable debug logs in these tests
	config.NewLoggerLevelCustom("INFO", "/var/log/datadog/trace-agent.log")

	conf := config.NewDefaultAgentConfig()
	conf.APIKeys = append(conf.APIKeys, "")
	agent := NewAgent(conf)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agent.Process(fixtures.RandomTrace())
	}
}

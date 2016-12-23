package main

import (
	"math/rand"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
)

func BenchmarkAgentTraceProcessing(b *testing.B) {
	// Disable debug logs in these tests
	config.NewLoggerLevelCustom("INFO", "/var/log/datadog/trace-agent.log")

	// TODO the seed must be passed using --seed flag so that executions
	// are random but can be easily reproduced; using the default Seed(1)
	// as a placeholder
	rand.Seed(1)

	conf := config.NewDefaultAgentConfig()
	conf.APIKeys = append(conf.APIKeys, "")
	agent := NewAgent(conf)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agent.Process(fixtures.RandomTrace())
	}
}

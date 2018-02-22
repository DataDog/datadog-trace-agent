package filters

import (
	"github.com/DataDog/datadog-trace-agent/agent"
	"github.com/DataDog/datadog-trace-agent/config"
)

// Filter is the interface implemented by all span-filters
type Filter interface {
	Keep(*agent.Span) bool
}

// Setup returns a slice of all registered filters
func Setup(c *config.AgentConfig) []Filter {
	return []Filter{
		newResourceFilter(c),
	}
}

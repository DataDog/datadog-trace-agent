package filters

import (
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
)

// Filter is the interface implemented by all span-filters
type Filter interface {
	Keep(*model.Span) bool
}

// Setup returns a slice of all registered filters
func Setup(c *config.AgentConfig) []Filter {
	filters := []Filter{
		newResourceFilter(c),
	}

	return filters
}

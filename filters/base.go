package filters

import (
	"github.com/StackVista/stackstate-trace-agent/config"
	"github.com/StackVista/stackstate-trace-agent/model"
)

// Filter is the interface implemented by all span-filters
type Filter interface {
	Keep(*model.Span, *model.Trace) bool
}

// Setup returns a slice of all registered filters
func Setup(c *config.AgentConfig) []Filter {
	return []Filter{
		newResourceFilter(c),
		newTagReplacer(c),
	}
}

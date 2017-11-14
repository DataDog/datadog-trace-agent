package filters

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

// LevelFilter implements a filter based on span levels
type LevelFilter struct {
	cutoff model.SpanLevel
}

// Keep returns true if SpanLevel is at or above the cutoff level
func (f *LevelFilter) Keep(s *model.Span) bool {
	return s.Meets(f.cutoff)
}

// NewLevelFilter returns a new level filter
func NewLevelFilter(cutoff model.SpanLevel) Filter {
	return &LevelFilter{cutoff}
}

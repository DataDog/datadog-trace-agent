package filters

import (
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
)

var _ Filter = (*tagReplacer)(nil)

// tagReplacer is a filter which replaces tag values based on its
// settings. It keeps all spans.
type tagReplacer struct {
	// replace maps tag keys to one or more sets of replacements
	replace []*config.ReplaceRule
}

func newTagReplacer(c *config.AgentConfig) *tagReplacer {
	return &tagReplacer{replace: c.ReplaceTags}
}

// Keep implements Filter.
func (f tagReplacer) Keep(root *model.Span, trace *model.Trace) bool {
	for _, rule := range f.replace {
		key, str, re := rule.Name, rule.Repl, rule.Re
		for _, s := range *trace {
			switch key {
			case "*":
				for k := range s.Meta {
					s.Meta[k] = re.ReplaceAllString(s.Meta[k], str)
				}
				s.Resource = re.ReplaceAllString(s.Resource, str)
			case "resource.name":
				s.Resource = re.ReplaceAllString(s.Resource, str)
			default:
				if s.Meta == nil {
					continue
				}
				if _, ok := s.Meta[key]; !ok {
					continue
				}
				s.Meta[key] = re.ReplaceAllString(s.Meta[key], str)
			}
		}
	}
	// always return true as the goal of this filter is only to mutate data
	return true
}

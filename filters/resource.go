package filters

import (
	"regexp"

	"github.com/DataDog/datadog-trace-agent/agent"
	"github.com/DataDog/datadog-trace-agent/config"

	log "github.com/cihub/seelog"
)

// ResourceFilter implements a resource-based filter
type resourceFilter struct {
	blacklist []*regexp.Regexp
}

// Keep returns true if Span.Resource doesn't match any of the filter's rules
func (f *resourceFilter) Keep(t *agent.Span) bool {
	for _, entry := range f.blacklist {
		if entry.MatchString(t.Resource) {
			return false
		}
	}

	return true
}

func newResourceFilter(conf *config.AgentConfig) Filter {
	blacklist := compileRules(conf.Ignore["resource"])

	return &resourceFilter{blacklist}
}

func compileRules(entries []string) []*regexp.Regexp {
	blacklist := make([]*regexp.Regexp, 0, len(entries))

	for _, entry := range entries {
		rule, err := regexp.Compile(entry)

		if err != nil {
			log.Errorf("invalid resource filter: %q", entry)
			continue
		}

		blacklist = append(blacklist, rule)
	}

	return blacklist
}

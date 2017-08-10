package main

import (
	"regexp"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	log "github.com/cihub/seelog"
)

// Filter handles Span filtering
type Filter interface {
	Keep(t *model.Span) bool
}

// ResourceFilter does resource-based filtering
type ResourceFilter struct {
	blacklist []*regexp.Regexp
}

// NewResourceFilter returns a ResourceFilter holding compiled regexes
func NewResourceFilter(conf *config.AgentConfig) *ResourceFilter {
	blacklist := compileRules(conf.ResourceBlacklist)

	return &ResourceFilter{blacklist}
}

// Keep returns true if Span.Resource doesn't match any of the filter's rules
func (f *ResourceFilter) Keep(t *model.Span) bool {
	for _, entry := range f.blacklist {
		if entry.MatchString(t.Resource) {
			return false
		}
	}

	return true
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

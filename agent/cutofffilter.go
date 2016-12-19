package main

import (
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	log "github.com/cihub/seelog"
)

// CutoffFilter generates meaningul resource for spans
type CutoffFilter struct {
	in  chan model.Trace
	out chan model.Trace

	conf *config.AgentConfig
}

// NewCutoffFilter creates a new CutoffFilter ready to be started
func NewCutoffFilter(in chan model.Trace, conf *config.AgentConfig) *CutoffFilter {
	return &CutoffFilter{
		in:   in,
		out:  make(chan model.Trace),
		conf: conf,
	}
}

// Run starts doing some quantizing
func (cf *CutoffFilter) Run() {
	for trace := range cf.in {
		root := trace.GetRoot()
		if root == nil {
			log.Debugf("skipping empty trace, has no spans, unable to find root")
			continue
		}

		end := root.End()
		now := model.Now()
		if now > end+cf.conf.OldestSpanCutoff {
			log.Debugf("trace was blocked because it is too old cutoff=%d now=%d end=%d root: %v", cf.conf.OldestSpanCutoff/1e9, now/1e9, end/1e9, root)
			statsd.Client.Count("trace_agent.concentrator.late_trace", 1, nil, 1)
			continue
		}

		log.Debugf("trace was accepted because it is recent enough cutoff=%d now=%d end=%d root: %v", cf.conf.OldestSpanCutoff/1e9, now/1e9, end/1e9, root)

		cf.out <- trace
	}

	close(cf.out)
}

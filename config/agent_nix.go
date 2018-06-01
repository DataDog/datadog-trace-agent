// +build !windows

package config

const (
	// DefaultLogFilePath is where the agent will write logs if not overriden in the conf
	DefaultLogFilePath = "/var/log/datadog/trace-agent.log"

	// DefaultConfigPersistPath is where the agent will attempt to persist server-sent config
	DefaultConfigPersistPath = "/var/run/datadog/trace_agent_config.gob"

	// Agent 5
	defaultDDAgentPy    = "/opt/datadog-agent/embedded/bin/python"
	defaultDDAgentPyEnv = "PYTHONPATH=/opt/datadog-agent/agent"

	// Agent 6
	defaultDDAgentBin = "/opt/datadog-agent/bin/agent/agent"
)

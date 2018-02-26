// +build !windows

package config

const (
	// DefaultLogFilePath is where the agent will write logs if not overriden in the conf
	DefaultLogFilePath = "/var/log/datadog/trace-agent.log"

	// Agent 5
	defaultDDAgentPy    = "/opt/datadog-agent/embedded/bin/python"
	defaultDDAgentPyEnv = "PYTHONPATH=/opt/datadog-agent/agent"

	// Agent 6
	defaultDDAgentBin = "/opt/datadog-agent/bin/agent/agent"
)

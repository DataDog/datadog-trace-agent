// +build !windows

package config

const (
	// DefaultLogFilePath is where the agent will write logs if not overriden in the conf
	DefaultLogFilePath = "/var/log/stackstate/trace-agent.log"

	// Agent 5
	defaultDDAgentPy    = "/opt/stackstate-agent/embedded/bin/python"
	defaultDDAgentPyEnv = "PYTHONPATH=/opt/stackstate-agent/agent"

	// Agent 6
	defaultDDAgentBin = "/opt/stackstate-agent/bin/agent/agent"
)

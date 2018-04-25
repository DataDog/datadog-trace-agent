// +build !windows

package config

const (
	// DefaultLogFilePath is where the agent will write logs if not overriden in the conf
	DefaultLogFilePath = "/var/log/stackstate/trace-agent.log"

	// Agent 5
	defaultSTSAgentPy    = "/opt/stackstate-agent/embedded/bin/python"
	defaultSTSAgentPyEnv = "PYTHONPATH=/opt/stackstate-agent/agent"

	// Agent 6
	defaultSTSAgentBin = "/opt/stackstate-agent/bin/agent/agent"
)

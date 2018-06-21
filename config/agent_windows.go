package config

const (
	// DefaultLogFilePath is where the agent will write logs if not overriden in the conf
	DefaultLogFilePath = "c:\\programdata\\datadog\\logs\\trace-agent.log"

	// Agent 5
	defaultDDAgentPy    = "c:\\Program Files\\Datadog\\Datadog Agent\\embedded\\python.exe"
	defaultDDAgentPyEnv = "PYTHONPATH=c:\\Program Files\\Datadog\\Datadog Agent\\agent"

	// Agent 6
	defaultDDAgentBin = "c:\\Program Files\\Datadog\\Datadog Agent\\embedded\\agent.exe"
)

// agent5Config points to the default agent 5 configuration path. It is used
// as a fallback when no configuration is set and the new default is missing.
const agent5Config = "c:\\programdata\\datadog\\datadog.conf"

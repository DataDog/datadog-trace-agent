package config

const (
	// DefaultLogFilePath is where the agent will write logs if not overriden in the conf
	DefaultLogFilePath = "c:\\programdata\\datadog\\logs\\trace-agent.log"

	// DefaultConfigPersistPath is where the agent will attempt to persist server-sent config
	DefaultConfigPersistPath = "c:\\programdata\\datadog\\logs\\trace-agent.gob"

	// Agent 5
	defaultDDAgentPy    = "c:\\Program Files\\Datadog\\Datadog Agent\\embedded\\python.exe"
	defaultDDAgentPyEnv = "PYTHONPATH=c:\\Program Files\\Datadog\\Datadog Agent\\agent"

	// Agent 6
	defaultDDAgentBin = "c:\\Program Files\\Datadog\\Datadog Agent\\embedded\\agent.exe"
)

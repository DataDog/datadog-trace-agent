package config

const (
	// DefaultLogFilePath is where the agent will write logs if not overriden in the conf
	DefaultLogFilePath = "c:\\programdata\\stackstate\\logs\\trace-agent.log"

	// Agent 5
	defaultDDAgentPy    = "c:\\Program Files\\StackState\\StackState Agent\\embedded\\python.exe"
	defaultDDAgentPyEnv = "PYTHONPATH=c:\\Program Files\\StackState\\StackState Agent\\agent"

	// Agent 6
	defaultDDAgentBin = "c:\\Program Files\\StackState\\StackState Agent\\embedded\\agent.exe"
)

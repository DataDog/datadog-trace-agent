package config

const (
	// DefaultLogFilePath is where the agent will write logs if not overriden in the conf
	DefaultLogFilePath = "c:\\programdata\\stackstate\\logs\\trace-agent.log"

	// Agent 5
	defaultSTSAgentPy    = "c:\\Program Files\\StackState\\StackState Agent\\embedded\\python.exe"
	defaultSTSAgentPyEnv = "PYTHONPATH=c:\\Program Files\\StackState\\StackState Agent\\agent"

	// Agent 6
	defaultSTSAgentBin = "c:\\Program Files\\StackState\\StackState Agent\\embedded\\agent.exe"
)

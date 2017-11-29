// +build !windows

package main

import (
	"flag"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	_ "net/http/pprof"
)

func init() {
	// command-line arguments
	flag.StringVar(&opts.ddConfigFile, "ddconfig", "/etc/dd-agent/datadog.conf", "Classic agent config file location")
	// FIXME: merge all APM configuration into dd-agent/datadog.conf and deprecate the below flag
	flag.StringVar(&opts.configFile, "config", "/etc/datadog/trace-agent.ini", "Trace agent ini config file.")
	flag.StringVar(&opts.pidfilePath, "pid", "", "Path to set pidfile for process")
	flag.BoolVar(&opts.version, "version", false, "Show version information and exit")
	flag.BoolVar(&opts.info, "info", false, "Show info about running trace agent process and exit")

	// profiling arguments
	flag.StringVar(&opts.cpuprofile, "cpuprofile", "", "Write cpu profile to file")
	flag.StringVar(&opts.memprofile, "memprofile", "", "Write memory profile to `file`")

	flag.Parse()
}

// main is the main application entry point
func main() {
	exit := make(chan struct{})

	// Handle stops properly
	go func() {
		defer watchdog.LogOnPanic()
		handleSignal(exit)
	}()

	runAgent(exit)
}

// +build !windows

package main

import (
	"context"
	"flag"
	_ "net/http/pprof"

	"stackstate-trace-agent/watchdog"
)

func init() {
	// command-line arguments
	// TODO: load from the .yaml automatically if there
	flag.StringVar(&opts.configFile, "config", "/etc/sts-agent/stackstate.conf", "StackState Agent config file location")
	flag.StringVar(&opts.legacyConfigFile, "ddconfig", "/etc/sts-agent/trace-agent.ini", "Deprecated extra configuration option.")
	flag.StringVar(&opts.pidfilePath, "pid", "", "Path to set pidfile for process")
	flag.BoolVar(&opts.version, "version", false, "Show version information and exit")
	flag.BoolVar(&opts.info, "info", false, "Show info about running trace agent process and exit")

	// profiling arguments
	// TODO: remove it from regular stable build
	flag.StringVar(&opts.cpuprofile, "cpuprofile", "", "Write cpu profile to file")
	flag.StringVar(&opts.memprofile, "memprofile", "", "Write memory profile to `file`")

	flag.Parse()
}

// main is the main application entry point
func main() {
	ctx, cancelFunc := context.WithCancel(context.Background())

	// Handle stops properly
	go func() {
		defer watchdog.LogOnPanic()
		handleSignal(cancelFunc)
	}()

	runAgent(ctx)
}

// +build !windows

package main

import (
	"context"
	_ "net/http/pprof"

	"github.com/DataDog/datadog-trace-agent/watchdog"
)

const defaultConfigPath = "/opt/datadog-agent/etc/datadog.yaml"

func registerOSSpecificFlags() {}

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

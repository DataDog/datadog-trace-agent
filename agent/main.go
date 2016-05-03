package main

import (
	"flag"
	"fmt"
	"math/rand"
	_ "net/http/pprof" // register debugger
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/statsd"
	log "github.com/cihub/seelog"
)

// handleSignal closes a channel to exit cleanly from routines
func handleSignal(exit chan struct{}) {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan)
	for signal := range sigChan {
		switch signal {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Info("Received interruption signal")
			close(exit)
		}
	}
}

// opts are the command-line options
var opts struct {
	configFile string
	debug      bool
	topology   bool
}

// main is the entrypoint of our code
func main() {
	flag.StringVar(&opts.configFile, "config", "/etc/datadog/trace-agent.ini", "Trace agent ini config file.")
	flag.BoolVar(&opts.debug, "debug", false, "Turn on debug mode")
	flag.BoolVar(&opts.topology, "topology", false, "Use TCP conns info to draw network topology")
	flag.Parse()

	// Instantiate the config
	conf, err := config.New(opts.configFile)
	if err != nil {
		panic(err)
	}

	// Initialize logging
	err = config.NewLoggerLevel(opts.debug)
	if err != nil {
		panic(fmt.Errorf("error with logger: %v", err))
	}
	defer log.Flush()

	// Initialize dogstatsd client
	err = statsd.Configure(conf, "dogstatsd")
	if err != nil {
		panic(fmt.Sprintf("Error configuring dogstatsd: %v", err))
	}

	// Seed rand
	rand.Seed(time.Now().UTC().UnixNano())

	agentConf, err := config.NewAgentConfig(conf)
	if err != nil {
		panic(err)
	}
	agentConf.Topology = opts.topology

	agent := NewAgent(agentConf)

	// Handle stops properly
	go handleSignal(agent.exit)

	agent.Run()
}

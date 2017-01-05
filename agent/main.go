package main

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/statsd"
	log "github.com/cihub/seelog"

	_ "net/http/pprof"
)

// handleSignal closes a channel to exit cleanly from routines
func handleSignal(exit chan struct{}) {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan)
	for signal := range sigChan {
		switch signal {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Info("received interruption signal")
			close(exit)
		}
	}
}

// opts are the command-line options
var opts struct {
	ddConfigFile string
	configFile   string
	debug        bool
	logLevel     string
	version      bool
}

// version info sourced from build flags
var (
	Version   string
	BuildDate string
	GitCommit string
	GitBranch string
	GoVersion string
)

// versionString returns the version information filled in at build time
func versionString() string {
	var buf bytes.Buffer

	if Version != "" {
		fmt.Fprintf(&buf, "Version: %s\n", Version)
	}
	if GitCommit != "" {
		fmt.Fprintf(&buf, "Git hash: %s\n", GitCommit)
	}
	if GitBranch != "" {
		fmt.Fprintf(&buf, "Git branch: %s\n", GitBranch)
	}
	if BuildDate != "" {
		fmt.Fprintf(&buf, "Build date: %s\n", BuildDate)
	}
	if GoVersion != "" {
		fmt.Fprintf(&buf, "Go Version: %s\n", GoVersion)
	}

	return buf.String()
}

// main is the entrypoint of our code
func main() {
	// command-line arguments
	flag.StringVar(&opts.ddConfigFile, "ddconfig", "/etc/dd-agent/datadog.conf", "Classic agent config file location")
	// FIXME: merge all APM configuration into dd-agent/datadog.conf and deprecate the below flag
	flag.StringVar(&opts.configFile, "config", "/etc/datadog/trace-agent.ini", "Trace agent ini config file.")
	flag.BoolVar(&opts.debug, "debug", false, "Turn on debug mode")
	flag.BoolVar(&opts.version, "version", false, "Show version information and exit")

	// profiling arguments
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")
	flag.Parse()

	// start CPU profiling
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Critical(err)
		}
		pprof.StartCPUProfile(f)
		log.Info("CPU profiling started...")
		defer pprof.StopCPUProfile()
	}

	if opts.version {
		fmt.Print(versionString())
		os.Exit(0)
	}

	// Instantiate the config
	var agentConf *config.AgentConfig
	var err error

	// tolerate errors in reading the config files. some setups will use only environment variables
	// which is OK
	legacyConf, _ := config.New(opts.configFile)
	conf, _ := config.New(opts.ddConfigFile)
	agentConf, err = config.NewAgentConfig(conf, legacyConf)
	if err != nil {
		panic(err)
	}

	// Initialize logging
	level := agentConf.LogLevel
	if opts.debug {
		level = "debug"
	}

	err = config.NewLoggerLevelCustom(level, agentConf.LogFilePath)
	if err != nil {
		panic(fmt.Errorf("error with logger: %v", err))
	}
	defer log.Flush()

	// Initialize dogstatsd client
	err = statsd.Configure(agentConf)
	if err != nil {
		fmt.Printf("Error configuring dogstatsd: %v", err)
	}

	// Seed rand
	rand.Seed(time.Now().UTC().UnixNano())

	agent := NewAgent(agentConf)

	// Handle stops properly
	go handleSignal(agent.exit)

	agent.Run()

	// collect memory profile
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Critical("could not create memory profile: ", err)
		}

		// get up-to-date statistics
		runtime.GC()
		// Not using WriteHeapProfile but instead calling WriteTo to
		// make sure we pass debug=1 and resolve pointers to names.
		if err := pprof.Lookup("heap").WriteTo(f, 1); err != nil {
			log.Critical("could not write memory profile: ", err)
		}
		f.Close()
	}
}

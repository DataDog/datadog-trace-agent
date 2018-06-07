package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	_ "net/http/pprof"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-agent/pkg/pidfile"
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// handleSignal closes a channel to exit cleanly from routines
func handleSignal(onSignal func()) {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	for signo := range sigChan {
		switch signo {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Infof("received signal %d (%v)", signo, signo)
			onSignal()
			return
		default:
			log.Warnf("unhandled signal %d (%v)", signo, signo)
		}
	}
}

// die logs an error message and makes the program exit immediately.
func die(format string, args ...interface{}) {
	if opts.info || opts.version {
		// here, we've silenced the logger, and just want plain console output
		fmt.Printf(format, args...)
		fmt.Print("")
	} else {
		log.Errorf(format, args...)
		log.Flush()
	}
	os.Exit(1)
}

// opts are the command-line options
var opts struct {
	configFile       string
	legacyConfigFile string
	pidfilePath      string
	logLevel         string
	version          bool
	info             bool
	cpuprofile       string
	memprofile       string
}

const agentDisabledMessage = `trace-agent not enabled.
Set env var DD_APM_ENABLED=true or add
apm_enabled: true
to your datadog.conf file.
Exiting.`

// runAgent is the entrypoint of our code
func runAgent(ctx context.Context) {
	// configure a default logger before anything so we can observe initialization
	if opts.info || opts.version {
		log.UseLogger(log.Disabled)
	} else {
		SetupDefaultLogger()
		defer log.Flush()
	}

	defer watchdog.LogOnPanic()

	// start CPU profiling
	if opts.cpuprofile != "" {
		f, err := os.Create(opts.cpuprofile)
		if err != nil {
			log.Critical(err)
		}
		pprof.StartCPUProfile(f)
		log.Info("CPU profiling started...")
		defer pprof.StopCPUProfile()
	}

	if opts.version {
		fmt.Print(info.VersionString())
		return
	}

	if !opts.info && opts.pidfilePath != "" {
		err := pidfile.WritePID(opts.pidfilePath)
		if err != nil {
			log.Errorf("Error while writing PID file, exiting: %v", err)
			os.Exit(1)
		}

		log.Infof("pid '%d' written to pid file '%s'", os.Getpid(), opts.pidfilePath)
		defer func() {
			// remove pidfile if set
			os.Remove(opts.pidfilePath)
		}()
	}

	// Instantiate the config
	var err error
	// trace-agent configuration
	var agentConf *config.AgentConfig
	// Agent 6 datadog.yaml config
	var yamlConf *config.YamlAgentConfig
	// Agent 5 datadog.conf config
	var conf *config.File
	// deprecated Agent 5 trace-agent.ini config
	var legacyConf *config.File

	if filepath.Ext(opts.configFile) == ".conf" || filepath.Ext(opts.configFile) == ".ini" {
		conf, err = config.NewIniIfExists(opts.configFile)
		if err != nil {
			log.Criticalf("Error reading datadog.conf: %s", err)
		}
		if conf != nil {
			log.Infof("Loading configuration from %s", opts.configFile)
		}
	} else if filepath.Ext(opts.configFile) == ".yaml" {
		yamlConf, err = config.NewYamlIfExists(opts.configFile)
		if err != nil {
			log.Criticalf("Error reading datadog.yaml: %s", err)
		}
		if conf != nil {
			log.Infof("Loading configuration from %s", opts.configFile)
		}
	} else {
		log.Errorf("Configuration file '%s' not supported, it must be a .yaml or .ini file. File ignored.", opts.configFile)
	}

	legacyConf, err = config.NewIniIfExists(opts.legacyConfigFile)
	if err != nil {
		log.Errorf("error reading %s: %s", opts.legacyConfigFile, err)
	}
	if legacyConf != nil {
		log.Errorf("using legacy configuration from %s, -ddconfig option is deprecated and will be removed in future versions", opts.legacyConfigFile)
	}

	agentConf, err = config.NewAgentConfig(conf, legacyConf, yamlConf)
	if err != nil {
		die("%v", err)
	}

	err = info.InitInfo(agentConf) // for expvar & -info option
	if err != nil {
		panic(err)
	}

	if opts.info {
		if err := info.Info(os.Stdout, agentConf); err != nil {
			os.Stdout.WriteString(fmt.Sprintf("failed to print info: %s\n", err))
			os.Exit(1)
		}
		return
	}

	// Exit if tracing is not enabled
	if !agentConf.Enabled {
		log.Info(agentDisabledMessage)

		// a sleep is necessary to ensure that supervisor registers this process as "STARTED"
		// If the exit is "too quick", we enter a BACKOFF->FATAL loop even though this is an expected exit
		// http://supervisord.org/subprocess.html#process-states
		time.Sleep(5 * time.Second)
		return
	}

	// Initialize logging (replacing the default logger). No need
	// to defer log.Flush, it was already done when calling
	// "SetupDefaultLogger" earlier.
	logLevel, ok := log.LogLevelFromString(strings.ToLower(agentConf.LogLevel))
	if !ok {
		logLevel = log.InfoLvl
	}
	duration := 10 * time.Second
	if !agentConf.LogThrottlingEnabled {
		duration = 0
	}
	err = SetupLogger(logLevel, agentConf.LogFilePath, duration, 10)
	if err != nil {
		die("cannot create logger: %v", err)
	}

	// Initialize dogstatsd client
	err = statsd.Configure(agentConf)
	if err != nil {
		die("cannot configure dogstatsd: %v", err)
	}

	// count the number of times the agent started
	statsd.Client.Count("datadog.trace_agent.started", 1, []string{
		"version:" + info.Version,
	}, 1)

	// Seed rand
	rand.Seed(time.Now().UTC().UnixNano())

	agent := NewAgent(ctx, agentConf)

	log.Infof("trace-agent running on host %s", agentConf.HostName)
	agent.Run()

	// collect memory profile
	if opts.memprofile != "" {
		f, err := os.Create(opts.memprofile)
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

package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	_ "net/http/pprof"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-agent/pkg/pidfile"
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/flags"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/osutil"
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

const agentDisabledMessage = `trace-agent not enabled.
Set env var DD_APM_ENABLED=true or add
apm_enabled: true
to your datadog.conf file.
Exiting.`

// runAgent is the entrypoint of our code
func runAgent(ctx context.Context) {
	// configure a default logger before anything so we can observe initialization
	if flags.Info || flags.Version {
		log.UseLogger(log.Disabled)
	} else {
		SetupDefaultLogger()
		defer log.Flush()
	}

	defer watchdog.LogOnPanic()

	// start CPU profiling
	if flags.CPUProfile != "" {
		f, err := os.Create(flags.CPUProfile)
		if err != nil {
			log.Critical(err)
		}
		pprof.StartCPUProfile(f)
		log.Info("CPU profiling started...")
		defer pprof.StopCPUProfile()
	}

	if flags.Version {
		fmt.Print(info.VersionString())
		return
	}

	if !flags.Info && flags.PIDFilePath != "" {
		err := pidfile.WritePID(flags.PIDFilePath)
		if err != nil {
			log.Errorf("Error while writing PID file, exiting: %v", err)
			os.Exit(1)
		}

		log.Infof("pid '%d' written to pid file '%s'", os.Getpid(), flags.PIDFilePath)
		defer func() {
			// remove pidfile if set
			os.Remove(flags.PIDFilePath)
		}()
	}

	cfg, err := config.Load(flags.ConfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			osutil.Exitf("%v", err)
		}
	} else {
		log.Infof("Loaded configuration: %s", cfg.ConfigPath)
	}
	cfg.LoadEnv()
	if err := cfg.Validate(); err != nil {
		osutil.Exitf("%v", err)
	}

	err = info.InitInfo(cfg) // for expvar & -info option
	if err != nil {
		panic(err)
	}

	if flags.Info {
		if err := info.Info(os.Stdout, cfg); err != nil {
			os.Stdout.WriteString(fmt.Sprintf("failed to print info: %s\n", err))
			os.Exit(1)
		}
		return
	}

	// Exit if tracing is not enabled
	if !cfg.Enabled {
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
	logLevel, ok := log.LogLevelFromString(strings.ToLower(cfg.LogLevel))
	if !ok {
		logLevel = log.InfoLvl
	}
	duration := 10 * time.Second
	if !cfg.LogThrottlingEnabled {
		duration = 0
	}
	err = SetupLogger(logLevel, cfg.LogFilePath, duration, 10)
	if err != nil {
		osutil.Exitf("cannot create logger: %v", err)
	}

	// Initialize dogstatsd client
	err = statsd.Configure(cfg)
	if err != nil {
		osutil.Exitf("cannot configure dogstatsd: %v", err)
	}

	// count the number of times the agent started
	statsd.Client.Count("datadog.trace_agent.started", 1, []string{
		"version:" + info.Version,
	}, 1)

	// Seed rand
	rand.Seed(time.Now().UTC().UnixNano())

	agent := NewAgent(ctx, cfg)

	log.Infof("trace-agent running on host %s", cfg.Hostname)
	agent.Run()

	// collect memory profile
	if flags.MemProfile != "" {
		f, err := os.Create(flags.MemProfile)
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

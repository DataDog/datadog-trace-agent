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
	"strings"
	"syscall"
	"time"

	log "github.com/cihub/seelog"
	_ "net/http/pprof"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// handleSignal closes a channel to exit cleanly from routines
func handleSignal(exit chan struct{}) {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	for signo := range sigChan {
		switch signo {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Infof("received signal %d (%v)", signo, signo)
			close(exit)
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
	ddConfigFile string
	configFile   string
	logLevel     string
	version      bool
	info         bool
	cpuprofile   string
	memprofile   string
}

// version info sourced from build flags
var (
	Version   string
	GitCommit string
	GitBranch string
	BuildDate string
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

const agentDisabledMessage = `trace-agent not enabled.
Set env var DD_APM_ENABLED=true or add
apm_enabled: true
to your datadog.conf file.
Exiting.`

func init() {
	// command-line arguments
	flag.StringVar(&opts.ddConfigFile, "ddconfig", "/etc/dd-agent/datadog.conf", "Classic agent config file location")
	// FIXME: merge all APM configuration into dd-agent/datadog.conf and deprecate the below flag
	flag.StringVar(&opts.configFile, "config", "/etc/datadog/trace-agent.ini", "Trace agent ini config file.")
	flag.BoolVar(&opts.version, "version", false, "Show version information and exit")
	flag.BoolVar(&opts.info, "info", false, "Show info about running trace agent process and exit")

	// profiling arguments
	flag.StringVar(&opts.cpuprofile, "cpuprofile", "", "Write cpu profile to file")
	flag.StringVar(&opts.memprofile, "memprofile", "", "Write memory profile to `file`")

	flag.Parse()
}

// main is the entrypoint of our code
func main() {
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
		fmt.Print(versionString())
		return
	}

	// Instantiate the config
	var agentConf *config.AgentConfig
	var err error

	// if a configuration file cannot be loaded, log an error but do not
	// panic since the agent can be configured with environment variables
	// only.
	legacyConf, err := config.NewIfExists(opts.configFile)
	if err != nil {
		log.Errorf("%s: %v", opts.configFile, err)
		log.Warnf("ignoring %s", opts.configFile)
	}
	if legacyConf != nil {
		log.Infof("using legacy configuration from %s", opts.configFile)
	}

	conf, err := config.NewIfExists(opts.ddConfigFile)
	if err != nil {
		log.Errorf("%s: %v", opts.ddConfigFile, err)
		log.Warnf("ignoring %s", opts.ddConfigFile)
	}
	if conf != nil {
		log.Infof("using configuration from %s", opts.ddConfigFile)
	}

	agentConf, err = config.NewAgentConfig(conf, legacyConf)
	if err != nil {
		die("%v", err)
	}

	err = initInfo(agentConf) // for expvar & -info option
	if err != nil {
		panic(err)
	}

	if opts.info {
		if err := Info(os.Stdout, agentConf); err != nil {
			// need not display the error, Info should do it already
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

	// Seed rand
	rand.Seed(time.Now().UTC().UnixNano())

	agent := NewAgent(agentConf)

	// Handle stops properly
	go func() {
		defer watchdog.LogOnPanic()
		handleSignal(agent.exit)
	}()

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

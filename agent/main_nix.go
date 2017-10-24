// +build !windows

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

// main is the main application entry point
func main() {
	runAgent()
}
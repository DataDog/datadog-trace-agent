package profile

import (
	"fmt"
	"github.com/DataDog/datadog-trace-agent/statsd"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
)

/*
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
#include <stdlib.h>
*/
import "C"

var ticksPerUsec float64

func init() {
	if runtime.GOOS == "linux" {
		// Scale according to the number of ticks per sec. On most implementations
		// it's 100 but technically, it could be different. The values returned
		// by /proc/pid/stat should be divided by this to be seconds.
		ticksPerUsec = float64(C.sysconf(C._SC_CLK_TCK)) / 1e6
	}
}

const (
	utimeIndex  = 13
	stimeIndex  = 14
	cutimeIndex = 15
	cstimeIndex = 16
	cnswapIndex = 36
)

// CPUInfo contains basic information about CPU usage.
type CPUInfo struct {
	Utime  int64 // Utime is the user time spent by this process
	Stime  int64 // Stime it the system time spent by this process
	Cutime int64 // Cutime is the user time spent by children processes
	Cstime int64 // Cstime is the system time spent by children processes
}

// IntakeStats contains traces intake statistics.
type IntakeStats struct {
	Payloads int64 // Payloads is the total number of traces payloads processed
	Traces   int64 // Traces is the total number of traces processed
	Spans    int64 // Spans is the total number of spans processed
	Bytes    int64 // Bytes is the total number of bytes (raw size of payloads) processed
}

// GetCPUInfo returns information about this process CPU usage.
// Only works on systems supporting /proc, tested on Linux only.
func GetCPUInfo() (*CPUInfo, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("os '%s' does not support CPU info", runtime.GOOS)
	}

	// There are functions in https://golang.org/pkg/os/#ProcessState to get
	// process info but they all refer to children as here, we want to get
	// info about this process.
	path := fmt.Sprintf("/proc/%d/stat", os.Getpid())
	content, err := ioutil.ReadFile(path)
	if err != nil && len(content) > 0 {
		return nil, fmt.Errorf("unable to read CPU info from '%s': %v", path, err)
	}
	fields := strings.Split(string(content), " ")
	if len(fields) <= cnswapIndex {
		return nil, fmt.Errorf("not enough (%d) fields in '%s': %v", len(fields), string(content), err)
	}
	var ci CPUInfo
	utime, err := strconv.Atoi(fields[utimeIndex])
	if err != nil {
		return nil, err
	}
	ci.Utime += int64(utime)
	stime, err := strconv.Atoi(fields[stimeIndex])
	if err != nil {
		return nil, err
	}
	ci.Stime += int64(stime)
	cutime, err := strconv.Atoi(fields[cutimeIndex])
	if err != nil {
		return nil, err
	}
	ci.Cutime += int64(cutime)
	cstime, err := strconv.Atoi(fields[cstimeIndex])
	if err != nil {
		return nil, err
	}
	ci.Cstime += int64(cstime)
	if err != nil {
		return &ci, nil
	}

	return &ci, nil
}

// TotalCPU returns the total number of ticks in a CPUInfo struct.
func TotalCPU(ci *CPUInfo) int64 {
	return ci.Utime + ci.Stime + ci.Cutime + ci.Cstime
}

var lastCPUInfo *CPUInfo
var lastIntakeStats IntakeStats

// CPUStatsd traces stats on a per-CPU used basis. While raw CPU stats
// can be obtained easily using standard Datadog tools, what we're searching
// is "how many CPU cycles does a payload/trace/span/byte costs."
// It is possible to calculate this later in dashboards, but if we know we're
// tracking this, let's do it upstream.
func CPUStatsd(stats IntakeStats, tags []string) {
	if ticksPerUsec <= 0 {
		return
	}
	ci, err := GetCPUInfo()
	if err != nil {
		log.Debugf("unable to get CPUInfo: %v", err)
	}
	if lastCPUInfo == nil {
		lastCPUInfo = ci
		lastIntakeStats = stats
		return
	}
	ticks := float64(TotalCPU(ci) - TotalCPU(lastCPUInfo))
	if ticks <= 0 {
		return
	}
	usecs := ticks / ticksPerUsec

	payloads := float64(stats.Payloads - lastIntakeStats.Payloads)
	if payloads >= 0 {
		statsd.Client.Gauge("trace_agent.profile.cpu.usec_per_payload", usecs/payloads, tags, 1)
	}
	traces := float64(stats.Traces - lastIntakeStats.Traces)
	if traces >= 0 {
		statsd.Client.Gauge("trace_agent.profile.cpu.usec_per_trace", usecs/traces, tags, 1)
	}
	spans := float64(stats.Spans - lastIntakeStats.Spans)
	if spans >= 0 {
		statsd.Client.Gauge("trace_agent.profile.cpu.usec_per_span", usecs/spans, tags, 1)
	}
	bytes := float64(stats.Bytes - lastIntakeStats.Bytes)
	if bytes >= 0 {
		statsd.Client.Gauge("trace_agent.profile.cpu.usec_per_byte", usecs/bytes, tags, 1)
	}

	lastCPUInfo = ci
	lastIntakeStats = stats
}

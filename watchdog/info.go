package watchdog

import (
	"os"
	"runtime"
	"time"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/process"
)

// CPUInfo contains very basic CPU information
type CPUInfo struct {
	// UserAvg is the average of the user CPU usage since last time
	// it was polled. 0 means "not used at all" and 1 means "1 CPU was
	// totally full for that period". So it might be greater than 1 if
	// the process is monopolizing several cores.
	UserAvg float64
}

// MemInfo contains very basic memory information
type MemInfo struct {
	// Alloc is the number of bytes allocated and not yet freed
	// as described in runtime.MemStats.Alloc
	Alloc uint64
	// AllocPerSec is the average number of bytes allocated, per second,
	// since last time this function was called.
	AllocPerSec float64
}

// ProcessInfo is used to query CPU and Mem info, it keeps data from
// the previous calls to calculate averages. It is not thread safe.
type ProcessInfo struct {
	p *process.Process

	lastCPUTime time.Time
	lastCPUUser float64
	lastCPU     CPUInfo

	lastMemTime       time.Time
	lastMemTotalAlloc uint64
	lastMem           MemInfo
}

// globalProcessInfo is a global default object one can safely use
// if only one goroutine is polling for CPU() and Mem()
var globalProcessInfo *ProcessInfo

func init() {
	var err error
	globalProcessInfo, err = NewProcessInfo()
	if err != nil {
		log.Errorf("unable to create global Process: %v", err)
	}
}

// NewProcessInfo creates a new ProcessInfo referring to the current running program.
func NewProcessInfo() (*ProcessInfo, error) {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		log.Debugf("unable to create Process: %v", err)
		return nil, err
	}
	return &ProcessInfo{p: p}, nil
}

// CPU returns basic CPU info
func (pi *ProcessInfo) CPU() CPUInfo {
	now := time.Now()
	dt := now.Sub(pi.lastCPUTime)
	if dt <= 0 {
		return pi.lastCPU // shouldn't happen unless time decreases or back to back calls
	}
	times, err := pi.p.Times()
	if err != nil {
		log.Debugf("unable to get CPU times: %v", err)
		return pi.lastCPU
	}
	pi.lastCPUTime = now
	dua := times.User - pi.lastCPUUser
	pi.lastCPUUser = times.User
	if dua <= 0 {
		pi.lastCPU.UserAvg = 0 // shouldn't happen, but make sure result is always > 0
	} else {
		pi.lastCPU.UserAvg = float64(time.Second) * dua / float64(dt)
		pi.lastCPUUser = times.User
	}

	return pi.lastCPU
}

// Mem returns basic memory information
func (pi *ProcessInfo) Mem() MemInfo {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	ret := MemInfo{Alloc: ms.Alloc, AllocPerSec: pi.lastMem.AllocPerSec}

	now := time.Now()
	dt := now.Sub(pi.lastMemTime)
	if dt <= 0 {
		return pi.lastMem // shouldn't happen unless time decreases or back to back calls
	}
	pi.lastMemTime = now
	dta := int64(ms.TotalAlloc) - int64(pi.lastMemTotalAlloc)
	pi.lastMemTotalAlloc = ms.TotalAlloc
	if dta <= 0 {
		pi.lastMem.AllocPerSec = 0 // shouldn't happen, but make sure result is always > 0
	} else {
		pi.lastMem.AllocPerSec = float64(time.Second) * float64(dta) / float64(dt)
	}
	ret.AllocPerSec = pi.lastMem.AllocPerSec

	return ret
}

// CPU returns basic CPU info.
// Uses a global shared struct to store information, therefore not thread safe.
func CPU() CPUInfo {
	if globalProcessInfo == nil {
		return CPUInfo{}
	}
	return globalProcessInfo.CPU()
}

// Mem returns basic memory information
// Uses a global shared struct to store information, therefore not thread safe.
func Mem() MemInfo {
	if globalProcessInfo == nil {
		return MemInfo{}
	}
	return globalProcessInfo.Mem()
}

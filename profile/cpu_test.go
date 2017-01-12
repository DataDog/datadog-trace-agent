package profile

import (
	"github.com/stretchr/testify/assert"
	"runtime"
	"testing"
)

const (
	testCycles = 100000000
)

func TestTotalCPU(t *testing.T) {
	assert := assert.New(t)

	ci := CPUInfo{Utime: 1, Stime: 2, Cutime: 3, Cstime: 4}
	assert.Equal(10, TotalCPU(&ci), "bad CPU total")
}

func TestGetCPUInfo(t *testing.T) {
	assert := assert.New(t)
	if runtime.GOOS == "linux" {
		ci, err := GetCPUInfo()
		assert.Nil(err, "unable to get CPU Info (note: this only works on UNIX machines supporting /proc/pid/stat")
		for i := 0; i < testCycles && TotalCPU(ci) == 0; i++ {
			// normally, the test bootstrap should consume enough CPU cycles,
			// but just in case, computing random stuff to increase numbers
			ci, err = GetCPUInfo()
			assert.Nil(err, "unable to get CPU Info (note: this only works on UNIX machines supporting /proc/pid/stat")
		}
		assert.NotEqual(0, TotalCPU(ci), "zero CPU usage found, does not make sense")
		ci2, err := GetCPUInfo()
		assert.Nil(err, "unable to get CPU Info (note: this only works on UNIX machines supporting /proc/pid/stat")
		for i := 0; i < testCycles && TotalCPU(ci) >= TotalCPU(ci2); i++ {
			// normally, the test bootstrap should consume enough CPU cycles,
			// but just in case, computing random stuff to increase numbers
			ci2, err = GetCPUInfo()
			assert.Nil(err, "unable to get CPU Info (note: this only works on UNIX machines supporting /proc/pid/stat")
		}
		assert.Condition(func() bool { return TotalCPU(ci) < TotalCPU(ci2) }, "CPU usage not increasing")
	} else {
		// unsupported
		ci, err := GetCPUInfo()
		assert.Nil(ci)
		assert.NotNil(err)
	}
}

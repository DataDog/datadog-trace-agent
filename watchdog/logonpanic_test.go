package watchdog

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	log "github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

var testLogBuf bytes.Buffer

func init() {
	logger, err := log.LoggerFromWriterWithMinLevelAndFormat(&testLogBuf, log.DebugLvl, "%Ns [%Level] %Msg")
	if err != nil {
		panic(err)
	}
	err = log.ReplaceLogger(logger)
	if err != nil {
		panic(err)
	}
}

func TestLogOnPanicMain(t *testing.T) {
	assert := assert.New(t)

	defer func() {
		r := recover()
		assert.NotNil(r, "panic should bubble up and be trapped here")
		assert.Contains(fmt.Sprintf("%v", r),
			"integer divide by zero",
			"divide by zero panic should be forwarded")
		msg := testLogBuf.String()
		assert.Contains(msg,
			"Unexpected error: runtime error: integer divide by zero",
			"divide by zero panic should be reported in log")
		assert.Contains(msg,
			"github.com/DataDog/datadog-trace-agent/watchdog.TestLogOnPanicMain",
			"log should contain a reference to this test func name as it displays the stack trace")
	}()
	defer LogOnPanic()
	zero := 0
	_ = 1 / zero
}

func TestLogOnPanicGoroutine(t *testing.T) {
	assert := assert.New(t)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer func() {
			r := recover()
			assert.NotNil(r, "panic should bubble up and be trapped here")
			assert.Contains(fmt.Sprintf("%v", r),
				"what could possibly go wrong?",
				"custom panic should be forwarded")
			msg := testLogBuf.String()
			assert.Contains(msg,
				"Unexpected error: what could possibly go wrong?",
				"custom panic should be reported in log")
			assert.Contains(msg,
				"github.com/DataDog/datadog-trace-agent/watchdog.TestLogOnPanicGoroutine",
				"log should contain a reference to this test func name as it displays the stack trace")
			wg.Done()
		}()
		defer LogOnPanic()
		panic("what could possibly go wrong?")
	}()
	defer func() {
		r := recover()
		assert.Nil(r, "this should trap no error at all, what we demonstrate here is that recover needs to be called on a per-goroutine base")
	}()
	wg.Wait()
}

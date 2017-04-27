package watchdog

import (
	"fmt"
	"runtime"

	"github.com/DataDog/datadog-trace-agent/statsd"
	log "github.com/cihub/seelog"
)

const shortErrMsgLen = 17 // 20 char max with tailing "..."

// shortMsg shortens the length of error message to avoid having high
// cardinality on "err:" tags
func shortErrMsg(msg string) string {
	if len(msg) <= shortErrMsgLen {
		return msg
	}
	return msg[:shortErrMsgLen] + "..."
}

// LogOnPanic catches panics and logs them on the fly. It also flushes
// the log file, ensuring the message appears. Then it propagates the panic
// so that the program flow remains unchanged.
func LogOnPanic() {
	if err := recover(); err != nil {
		// Full print of the trace in the logs
		buf := make([]byte, 4096)
		length := runtime.Stack(buf, false)
		stacktrace := string(buf[:length])
		msg := fmt.Sprintf("%v: %s\n%s", "Unexpected error", err, stacktrace)

		statsd.Client.Gauge("datadog.trace_agent.panic", 1, []string{
			"err:" + shortErrMsg(msg),
		}, 1)

		log.Error(msg)
		log.Flush()
		panic(err)
	}
}

// Go is a helper func that calls LogOnPanic and f in a goroutine.
func Go(f func()) {
	go func() {
		defer LogOnPanic()
		f()
	}()
}

package watchdog

import (
	"fmt"
	"runtime"

	log "github.com/cihub/seelog"
)

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

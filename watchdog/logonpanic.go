package watchdog

import (
	"fmt"
	"runtime"

	log "github.com/cihub/seelog"
)

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

package agent

import (
	"time"
)

// Now returns a timestamp in our nanoseconds default format
func Now() int64 {
	return time.Now().UnixNano()
}

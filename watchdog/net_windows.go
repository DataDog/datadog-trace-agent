package watchdog

import (
	"time"
)

// Net for windows returns basic network info without the number of connections.
func (pi *CurrentInfo) Net() NetInfo {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	now := time.Now()
	dt := now.Sub(pi.lastNetTime)
	if dt <= pi.cacheDelay {
		return pi.lastNet // don't query too often, cache a little bit
	}
	pi.lastNetTime = now

	return pi.lastNet
}

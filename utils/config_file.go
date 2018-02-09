package utils

import (
	"os"
	"time"
)

// PathExists returns a boolean indicating if the given path exists on the file system.
func PathExists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}

// GetDuration returns the duration of the provided value in seconds
func GetDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

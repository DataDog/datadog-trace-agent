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

// GetDuration returns the duration of the provided value
func GetDuration(value int) time.Duration {
	return time.Duration(value) * time.Second
}

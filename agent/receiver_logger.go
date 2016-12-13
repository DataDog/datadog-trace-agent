package main

import (
	"sync"

	log "github.com/cihub/seelog"
)

// these could be configurable, but fine with hardcoded for now
var maxPerInterval int64 = 10

type errorLogger struct {
	errors int64
	sync.Mutex
}

func (l *errorLogger) Errorf(format string, params ...interface{}) {
	l.Lock()
	defer l.Unlock()

	if l.errors < maxPerInterval {
		log.Errorf(format, params...)
	}
	if l.errors == maxPerInterval {
		log.Errorf("too many error messages to display, truncating output till next period")
	}

	l.errors++
}

func (l *errorLogger) Reset() {
	l.Lock()
	l.errors = 0
	l.Unlock()
}

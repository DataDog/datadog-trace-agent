package main

import (
	"expvar" // automatically publish `/debug/vars` on HTTP port
	"github.com/DataDog/datadog-trace-agent/config"
	"os"
	"sync"
	"time"
)

var (
	infoMu            sync.RWMutex
	infoReceiverStats ReceiverStats // only for the last minute
	infoConfig        config.AgentConfig
	start             time.Time
)

func init() {
	start = time.Now()
}

func publishUptime() interface{} {
	return int(time.Now().Sub(start) / time.Second)
}

func updateReceiverStats(rs ReceiverStats) {
	infoMu.Lock()
	infoReceiverStats = rs
	infoMu.Unlock()
}

func updateConf(conf *config.AgentConfig) {
	infoMu.Lock()
	infoConfig = *conf       // not a real deep copy but slices in conf should not change
	infoConfig.APIKeys = nil // should not be exported by JSON, but just to make sure
	infoMu.Unlock()
}

func publishReceiverStats() interface{} {
	infoMu.RLock()
	rs := infoReceiverStats
	infoMu.RUnlock()
	return rs
}

func publishConfig() interface{} {
	infoMu.RLock()
	c := infoConfig
	infoMu.RUnlock()
	return c
}

func init() {
	expvar.NewInt("pid").Set(int64(os.Getpid()))
	expvar.Publish("uptime", expvar.Func(publishUptime))
	expvar.Publish("receiver", expvar.Func(publishReceiverStats))
	expvar.Publish("config", expvar.Func(publishConfig))
}

package writer

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// StatsWriter ingests service metadata and flush them to the API.
type StatsWriter struct {
	endpoint Endpoint

	InStats <-chan []model.StatsBucket

	stats info.StatsWriterInfo

	exit   chan struct{}
	exitWG *sync.WaitGroup

	conf *config.AgentConfig
}

// NewStatsWriter returns a new writer for services.
func NewStatsWriter(conf *config.AgentConfig, InStats <-chan []model.StatsBucket) *StatsWriter {
	var endpoint Endpoint

	if conf.APIEnabled {
		client := NewClient(conf)
		endpoint = NewDatadogEndpoint(client, conf.APIEndpoint, "/api/v0.2/stats", conf.APIKey)
	} else {
		log.Info("API interface is disabled, flushing to /dev/null instead")
		endpoint = &NullEndpoint{}
	}

	return &StatsWriter{
		endpoint: endpoint,

		InStats: InStats,

		exit:   make(chan struct{}),
		exitWG: &sync.WaitGroup{},

		conf: conf,
	}
}

// Start starts the writer.
func (w *StatsWriter) Start() {
	go func() {
		defer watchdog.LogOnPanic()
		w.Run()
	}()
}

// Run runs the main loop of the writer goroutine. If flushes
// stats buckets once received from the concentrator.
func (w *StatsWriter) Run() {
	w.exitWG.Add(1)
	defer w.exitWG.Done()

	log.Debug("starting stats writer")

	updateInfoTicker := time.NewTicker(1 * time.Minute)
	defer updateInfoTicker.Stop()

	for {
		select {
		case stats := <-w.InStats:
			// TODO: have a buffer with replay abilities
			w.Flush(stats)
		case <-updateInfoTicker.C:
			go w.updateInfo()
		case <-w.exit:
			log.Info("exiting stats writer")
			return
		}
	}
}

// Stop stops the main Run loop.
func (w *StatsWriter) Stop() {
	close(w.exit)
	w.exitWG.Wait()
}

// Flush flushes received stats
func (w *StatsWriter) Flush(stats []model.StatsBucket) {
	if len(stats) == 0 {
		log.Debugf("no stats to flush")
		return
	}
	log.Debugf("going to flush stats buckets, %d buckets", len(stats))
	atomic.AddInt64(&w.stats.StatsBuckets, int64(len(stats)))

	statsPayload := &model.StatsPayload{
		HostName: w.conf.HostName,
		Env:      w.conf.DefaultEnv,
		Stats:    stats,
	}

	data, err := model.EncodeStatsPayload(statsPayload)
	if err != nil {
		log.Errorf("encoding issue: %v", err)
		return
	}

	headers := map[string]string{
		languageHeaderKey:  strings.Join(info.Languages(), "|"),
		"Content-Type":     "application/json",
		"Content-Encoding": "gzip",
	}

	atomic.AddInt64(&w.stats.Bytes, int64(len(data)))

	startFlush := time.Now()

	// Send the payload to the endpoint
	err = w.endpoint.Write(data, headers)

	flushTime := time.Since(startFlush)

	// TODO: if error, depending on why, replay later.
	if err != nil {
		atomic.AddInt64(&w.stats.Errors, 1)
		log.Errorf("failed to flush service payload, time:%s, size:%d bytes, error: %s", flushTime, len(data), err)
		return
	}

	log.Infof("flushed service payload to the API, time:%s, size:%d bytes", flushTime, len(data))
	statsd.Client.Gauge("datadog.trace_agent.stats_writer.flush_duration", flushTime.Seconds(), nil, 1)
	atomic.AddInt64(&w.stats.Payloads, 1)
}

func (w *StatsWriter) updateInfo() {
	var swInfo info.StatsWriterInfo

	// Load counters and reset them for the next flush
	swInfo.Payloads = atomic.SwapInt64(&w.stats.Payloads, 0)
	swInfo.StatsBuckets = atomic.SwapInt64(&w.stats.StatsBuckets, 0)
	swInfo.Bytes = atomic.SwapInt64(&w.stats.Bytes, 0)
	swInfo.Errors = atomic.SwapInt64(&w.stats.Errors, 0)

	statsd.Client.Count("datadog.trace_agent.stats_writer.payloads", int64(swInfo.Payloads), nil, 1)
	statsd.Client.Count("datadog.trace_agent.stats_writer.stats_buckets", int64(swInfo.StatsBuckets), nil, 1)
	statsd.Client.Count("datadog.trace_agent.stats_writer.bytes", int64(swInfo.Bytes), nil, 1)
	statsd.Client.Count("datadog.trace_agent.stats_writer.errors", int64(swInfo.Errors), nil, 1)

	info.UpdateStatsWriterInfo(swInfo)
}

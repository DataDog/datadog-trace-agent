package writer

import (
	"strings"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
)

// StatsWriter ingests stats buckets and flushes their aggregation to the API.
type StatsWriter struct {
	stats    info.StatsWriterInfo
	hostName string
	env      string
	conf     writerconfig.StatsWriterConfig
	InStats  <-chan []model.StatsBucket

	BaseWriter
}

// NewStatsWriter returns a new writer for services.
func NewStatsWriter(conf *config.AgentConfig, InStats <-chan []model.StatsBucket) *StatsWriter {
	writerConf := conf.StatsWriterConfig
	log.Infof("Stats writer initializing with config: %+v", writerConf)

	return &StatsWriter{
		hostName: conf.HostName,
		env:      conf.DefaultEnv,
		conf:     writerConf,
		InStats:  InStats,
		BaseWriter: *NewBaseWriter(conf, "/api/v0.2/stats", func(endpoint Endpoint) PayloadSender {
			return NewCustomQueuablePayloadSender(endpoint, writerConf.SenderConfig)
		}),
	}
}

// Start starts the writer.
func (w *StatsWriter) Start() {
	w.BaseWriter.Start()
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

	updateInfoTicker := time.NewTicker(w.conf.UpdateInfoPeriod)
	defer updateInfoTicker.Stop()

	// Monitor sender for events
	go func() {
		for event := range w.payloadSender.Monitor() {
			if event == nil {
				continue
			}

			switch event := event.(type) {
			case SenderSuccessEvent:
				log.Infof("flushed stat payload to the API, time:%s, size:%d bytes", event.SendStats.SendTime,
					len(event.Payload.Bytes))
				w.statsClient.Gauge("datadog.trace_agent.stats_writer.flush_duration",
					event.SendStats.SendTime.Seconds(), nil, 1)
				atomic.AddInt64(&w.stats.Payloads, 1)
			case SenderFailureEvent:
				log.Errorf("failed to flush stat payload, time:%s, size:%d bytes, error: %s",
					event.SendStats.SendTime, len(event.Payload.Bytes), event.Error)
				atomic.AddInt64(&w.stats.Errors, 1)
			case SenderRetryEvent:
				log.Errorf("retrying flush stat payload, retryNum: %d, delay:%s, error: %s",
					event.RetryNum, event.RetryDelay, event.Error)
				atomic.AddInt64(&w.stats.Retries, 1)
			default:
				log.Debugf("don't know how to handle event with type %T", event)
			}
		}
	}()

	for {
		select {
		case stats := <-w.InStats:
			w.handleStats(stats)
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
	w.BaseWriter.Stop()
}

func (w *StatsWriter) handleStats(stats []model.StatsBucket) {
	numStats := len(stats)

	// If no stats, we can't construct anything
	if numStats == 0 {
		return
	}
	log.Debugf("going to flush stats buckets, %d buckets", numStats)
	atomic.AddInt64(&w.stats.StatsBuckets, int64(numStats))

	statsPayload := &model.StatsPayload{
		HostName: w.hostName,
		Env:      w.env,
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

	payload := NewPayload(data, headers)

	w.payloadSender.Send(payload)
}

func (w *StatsWriter) updateInfo() {
	var swInfo info.StatsWriterInfo

	// Load counters and reset them for the next flush
	swInfo.Payloads = atomic.SwapInt64(&w.stats.Payloads, 0)
	swInfo.StatsBuckets = atomic.SwapInt64(&w.stats.StatsBuckets, 0)
	swInfo.Bytes = atomic.SwapInt64(&w.stats.Bytes, 0)
	swInfo.Retries = atomic.SwapInt64(&w.stats.Retries, 0)
	swInfo.Errors = atomic.SwapInt64(&w.stats.Errors, 0)

	w.statsClient.Count("datadog.trace_agent.stats_writer.payloads", int64(swInfo.Payloads), nil, 1)
	w.statsClient.Count("datadog.trace_agent.stats_writer.stats_buckets", int64(swInfo.StatsBuckets), nil, 1)
	w.statsClient.Count("datadog.trace_agent.stats_writer.bytes", int64(swInfo.Bytes), nil, 1)
	w.statsClient.Count("datadog.trace_agent.stats_writer.retries", int64(swInfo.Retries), nil, 1)
	w.statsClient.Count("datadog.trace_agent.stats_writer.errors", int64(swInfo.Errors), nil, 1)

	info.UpdateStatsWriterInfo(swInfo)
}

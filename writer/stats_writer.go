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

// StatsWriter ingests stats buckets and flushes them to the API.
type StatsWriter struct {
	BaseWriter

	// InStats is the stream of stat buckets to send out.
	InStats <-chan []model.StatsBucket

	// info contains various statistics about the writer, which are
	// occasionally sent as metrics to Datadog.
	info info.StatsWriterInfo

	// hostName specifies the resolved host name on which the agent is
	// running, to be sent as part of a stats payload.
	hostName string

	// env is environment this agent is configured with, to be sent as part
	// of the stats payload.
	env string

	conf writerconfig.StatsWriterConfig
}

// NewStatsWriter returns a new writer for stats.
func NewStatsWriter(conf *config.AgentConfig, InStats <-chan []model.StatsBucket) *StatsWriter {
	writerConf := conf.StatsWriterConfig
	log.Infof("Stats writer initializing with config: %+v", writerConf)

	bw := *NewBaseWriter(conf, "/api/v0.2/stats", func(endpoint Endpoint) PayloadSender {
		return NewCustomQueuablePayloadSender(endpoint, writerConf.SenderConfig)
	})
	return &StatsWriter{
		BaseWriter: bw,
		InStats:    InStats,
		hostName:   conf.HostName,
		env:        conf.DefaultEnv,
		conf:       writerConf,
	}
}

// Start starts the writer, awaiting stat buckets and flushing them.
func (w *StatsWriter) Start() {
	w.BaseWriter.Start()

	go func() {
		defer watchdog.LogOnPanic()
		w.Run()
	}()

	go func() {
		defer watchdog.LogOnPanic()
		w.monitor()
	}()
}

// Run runs the event loop of the writer's main goroutine. It reads stat buckets
// from InStats, builds stat payloads and sends them out using the base writer.
func (w *StatsWriter) Run() {
	w.exitWG.Add(1)
	defer w.exitWG.Done()

	log.Debug("starting stats writer")

	for {
		select {
		case stats := <-w.InStats:
			w.handleStats(stats)
		case <-w.exit:
			log.Info("exiting stats writer")
			return
		}
	}
}

// Stop stops the writer
func (w *StatsWriter) Stop() {
	close(w.exit)
	w.exitWG.Wait()

	// Closing the base writer, among other things, will close the
	// w.payloadSender.Monitor() channel, stoping the monitoring
	// goroutine.
	w.BaseWriter.Stop()
}

func (w *StatsWriter) handleStats(stats []model.StatsBucket) {
	numStats := len(stats)
	if numStats == 0 {
		return
	}

	log.Debugf("going to flush stats buckets, %d buckets", numStats)
	atomic.AddInt64(&w.info.StatsBuckets, int64(numStats))

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

	atomic.AddInt64(&w.info.Bytes, int64(len(data)))

	payload := NewPayload(data, headers)

	w.payloadSender.Send(payload)
}

// monitor runs the event loop of the writer's monitoring
// goroutine. It:
// - reads events from the payload sender's monitor channel, logs
//   them, send out statsd metrics, and updates the writer info
// - periodically dumps the writer info
func (w *StatsWriter) monitor() {
	monC := w.payloadSender.Monitor()

	infoTicker := time.NewTicker(w.conf.UpdateInfoPeriod)
	defer infoTicker.Stop()

	for {
		select {
		case e, ok := <-monC:
			if !ok {
				break
			}

			switch e := e.(type) {
			case SenderSuccessEvent:
				log.Infof("flushed stat payload to the API, time:%s, size:%d bytes", e.SendStats.SendTime,
					len(e.Payload.Bytes))
				w.statsClient.Gauge("datadog.trace_agent.stats_writer.flush_duration",
					e.SendStats.SendTime.Seconds(), nil, 1)
				atomic.AddInt64(&w.info.Payloads, 1)
			case SenderFailureEvent:
				log.Errorf("failed to flush stat payload, time:%s, size:%d bytes, error: %s",
					e.SendStats.SendTime, len(e.Payload.Bytes), e.Error)
				atomic.AddInt64(&w.info.Errors, 1)
			case SenderRetryEvent:
				log.Errorf("retrying flush stat payload, retryNum: %d, delay:%s, error: %s",
					e.RetryNum, e.RetryDelay, e.Error)
				atomic.AddInt64(&w.info.Retries, 1)
			default:
				log.Debugf("don't know how to handle event with type %T", e)
			}

		case <-infoTicker.C:
			var swInfo info.StatsWriterInfo

			// Load counters and reset them for the next flush
			swInfo.Payloads = atomic.SwapInt64(&w.info.Payloads, 0)
			swInfo.StatsBuckets = atomic.SwapInt64(&w.info.StatsBuckets, 0)
			swInfo.Bytes = atomic.SwapInt64(&w.info.Bytes, 0)
			swInfo.Retries = atomic.SwapInt64(&w.info.Retries, 0)
			swInfo.Errors = atomic.SwapInt64(&w.info.Errors, 0)

			w.statsClient.Count("datadog.trace_agent.stats_writer.payloads", int64(swInfo.Payloads), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.stats_buckets", int64(swInfo.StatsBuckets), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.bytes", int64(swInfo.Bytes), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.retries", int64(swInfo.Retries), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.errors", int64(swInfo.Errors), nil, 1)

			info.UpdateStatsWriterInfo(swInfo)
		}
	}
}

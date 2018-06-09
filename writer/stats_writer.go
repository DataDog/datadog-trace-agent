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
	payloads, nbStatBuckets, nbEntries := w.buildPayloads(stats, w.conf.MaxEntriesPerPayload)
	if len(payloads) == 0 {
		return
	}

	log.Debugf("going to flush %v entries in %v stat buckets in %v payloads",
		nbEntries, nbStatBuckets, len(payloads),
	)

	if len(payloads) > 1 {
		atomic.AddInt64(&w.info.Splits, 1)
	}
	atomic.AddInt64(&w.info.StatsBuckets, int64(nbStatBuckets))

	headers := map[string]string{
		languageHeaderKey:  strings.Join(info.Languages(), "|"),
		"Content-Type":     "application/json",
		"Content-Encoding": "gzip",
	}

	for _, p := range payloads {
		// synchronously send the payloads one after the other
		data, err := model.EncodeStatsPayload(p)
		if err != nil {
			log.Errorf("encoding issue: %v", err)
			return
		}

		payload := NewPayload(data, headers)
		w.payloadSender.Send(payload)

		atomic.AddInt64(&w.info.Bytes, int64(len(data)))
	}
}

type timeWindow struct {
	start, duration int64
}

// buildPayloads returns a set of payload to send out, each paylods guaranteed
// to have the number of stats buckets under the given maximum.
func (w *StatsWriter) buildPayloads(stats []model.StatsBucket, maxEntriesPerPayloads int) ([]*model.StatsPayload, int, int) {
	if len(stats) == 0 {
		return []*model.StatsPayload{}, 0, 0
	}

	// 1. Get an estimate of how many payloads we need, based on the total
	//    number of map entries (i.e.: sum of number of items in the stats
	//    bucket's count map).
	//    NOTE: we use the number of items in the count map as the
	//    reference, but in reality, what take place are the
	//    distributions. We are guaranteed the number of entries in the
	//    count map is > than the number of entries in the distributions
	//    maps, so the algorithm is correct, but indeed this means we could
	//    do better.
	nbEntries := 0
	for _, s := range stats {
		nbEntries += len(s.Counts)
	}

	if maxEntriesPerPayloads <= 0 || nbEntries < maxEntriesPerPayloads {
		// nothing to do, break early
		return []*model.StatsPayload{&model.StatsPayload{
			HostName: w.hostName,
			Env:      w.env,
			Stats:    stats,
		}}, len(stats), nbEntries
	}

	nbPayloads := nbEntries / maxEntriesPerPayloads
	if nbEntries%maxEntriesPerPayloads != 0 {
		nbPayloads++
	}

	// 2. Create a slice of nbPayloads maps, mapping a time window (stat +
	//    duration) to a stat bucket. We will build the payloads from these
	//    maps. This allows is to have one stat bucket per time window.
	pMaps := make([]map[timeWindow]model.StatsBucket, nbPayloads)
	for i := 0; i < nbPayloads; i++ {
		pMaps[i] = make(map[timeWindow]model.StatsBucket, nbPayloads)
	}

	// 3. Iterate over all entries of each stats. Add the entry to one of
	//    the payload container mappings, in a round robin fashion. In some
	//    edge cases, we can end up having the same entry in several
	//    inputted stat buckets. We must check that we never overwrite an
	//    entry in the new stats buckets but cleanly merge instead.
	i := 0
	for _, b := range stats {
		tw := timeWindow{b.Start, b.Duration}

		for ekey, e := range b.Counts {
			pm := pMaps[i%nbPayloads]
			newsb, ok := pm[tw]
			if !ok {
				newsb = model.NewStatsBucket(tw.start, tw.duration)
			}
			pm[tw] = newsb

			if _, ok := newsb.Counts[ekey]; ok {
				newsb.Counts[ekey].Merge(e)
			} else {
				newsb.Counts[ekey] = e
			}

			if _, ok := b.Distributions[ekey]; ok {
				if _, ok := newsb.Distributions[ekey]; ok {
					newsb.Distributions[ekey].Merge(b.Distributions[ekey])
				} else {
					newsb.Distributions[ekey] = b.Distributions[ekey]
				}
			}
			if _, ok := b.ErrDistributions[ekey]; ok {
				if _, ok := newsb.ErrDistributions[ekey]; ok {
					newsb.ErrDistributions[ekey].Merge(b.ErrDistributions[ekey])
				} else {
					newsb.ErrDistributions[ekey] = b.ErrDistributions[ekey]
				}
			}
			i++
		}
	}

	// 4. Create the nbPayloads payloads from the maps.
	nbStats := 0
	nbEntries = 0
	payloads := make([]*model.StatsPayload, 0, nbPayloads)
	for _, pm := range pMaps {
		pstats := make([]model.StatsBucket, 0, len(pm))
		for _, sb := range pm {
			pstats = append(pstats, sb)
			nbEntries += len(sb.Counts)
		}
		payloads = append(payloads, &model.StatsPayload{
			HostName: w.hostName,
			Env:      w.env,
			Stats:    pstats,
		})

		nbStats += len(pstats)
	}
	return payloads, nbStats, nbEntries
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
			swInfo.Splits = atomic.SwapInt64(&w.info.Splits, 0)
			swInfo.Errors = atomic.SwapInt64(&w.info.Errors, 0)

			w.statsClient.Count("datadog.trace_agent.stats_writer.payloads", int64(swInfo.Payloads), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.stats_buckets", int64(swInfo.StatsBuckets), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.bytes", int64(swInfo.Bytes), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.retries", int64(swInfo.Retries), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.splits", int64(swInfo.Splits), nil, 1)
			w.statsClient.Count("datadog.trace_agent.stats_writer.errors", int64(swInfo.Errors), nil, 1)

			info.UpdateStatsWriterInfo(swInfo)
		}
	}
}

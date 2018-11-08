package writer

import (
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
	"github.com/golang/protobuf/proto"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/model/collector"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
)

// CollectorTraceWriter ingests all traces and flushes them to the Collector.
type CollectorTraceWriter struct {
	stats    info.TraceWriterInfo
	hostName string
	env      string
	conf     writerconfig.TraceWriterConfig
	in       <-chan *model.Trace

	traces        []*collector.APITraceChunk
	spansInBuffer int

	sender payloadSender
	exit   chan struct{}
}

// NewCollectorTraceWriter returns a new writer for traces.
func NewCollectorTraceWriter(conf *config.AgentConfig, in <-chan *model.Trace) *CollectorTraceWriter {
	writerConf := conf.TraceWriterConfig
	log.Infof("Collector trace writer initializing with config: %+v", writerConf)
	endpoint := NewCollectorEndpoint(conf.CollectorAddr)

	return &CollectorTraceWriter{
		conf:     writerConf,
		hostName: conf.Hostname,
		env:      conf.DefaultEnv,

		traces: []*collector.APITraceChunk{},

		in: in,

		sender: newSender(endpoint, writerConf.SenderConfig),
		exit:   make(chan struct{}),
	}
}

// Start starts the writer.
func (w *CollectorTraceWriter) Start() {
	w.sender.Start()
	go func() {
		defer watchdog.LogOnPanic()
		w.Run()
	}()
}

// Run runs the main loop of the writer goroutine. It sends traces to the payload constructor, flushing it periodically
// and collects stats which are also reported periodically.
func (w *CollectorTraceWriter) Run() {
	defer close(w.exit)

	// for now, simply flush every x seconds
	flushTicker := time.NewTicker(w.conf.FlushPeriod)
	defer flushTicker.Stop()

	updateInfoTicker := time.NewTicker(w.conf.UpdateInfoPeriod)
	defer updateInfoTicker.Stop()

	// Monitor sender for events
	go func() {
		for event := range w.sender.Monitor() {
			switch event.typ {
			case eventTypeSuccess:
				log.Infof("flushed trace payload to the collector, time:%s, size:%d bytes", event.stats.sendTime,
					len(event.payload.bytes))
				statsd.Client.Gauge("datadog.trace_agent.collector_trace_writer.flush_duration",
					event.stats.sendTime.Seconds(), nil, 1)
				atomic.AddInt64(&w.stats.Payloads, 1)
			case eventTypeFailure:
				log.Errorf("failed to flush trace payload, time:%s, size:%d bytes, error: %s",
					event.stats.sendTime, len(event.payload.bytes), event.err)
				atomic.AddInt64(&w.stats.Errors, 1)
			case eventTypeRetry:
				log.Errorf("retrying flush trace payload, retryNum: %d, delay:%s, error: %s",
					event.retryNum, event.retryDelay, event.err)
				atomic.AddInt64(&w.stats.Retries, 1)
			default:
				log.Debugf("don't know how to handle event with type %T", event)
			}
		}
	}()

	log.Debug("starting collector trace writer")

	for {
		select {
		case trace := <-w.in:
			w.handleTrace(trace)
		case <-flushTicker.C:
			log.Debug("Flushing current traces")
			w.flush()
		case <-updateInfoTicker.C:
			go w.updateInfo()
		case <-w.exit:
			log.Info("exiting collector trace writer, flushing all remaining traces")
			w.flush()
			w.updateInfo()
			log.Info("Flushed. Exiting")
			return
		}
	}
}

// Stop stops the main Run loop.
func (w *CollectorTraceWriter) Stop() {
	w.exit <- struct{}{}
	<-w.exit
	w.sender.Stop()
}

func (w *CollectorTraceWriter) handleTrace(trace *model.Trace) {
	if trace == nil || len(*trace) == 0 {
		log.Debug("Ignoring empty trace")
		return
	}

	var n int

	if trace != nil {
		n += len(*trace)
	}

	if w.spansInBuffer > 0 && w.spansInBuffer+n > w.conf.MaxSpansPerPayload {
		// If we have data pending and adding the new data would overflow max spans per payload, force a flush
		w.flushDueToMaxSpansPerPayload()
	}

	w.appendTrace(trace)

	if n > w.conf.MaxSpansPerPayload {
		// If what we just added already goes over the limit, report this but lets carry on and flush
		atomic.AddInt64(&w.stats.SingleMaxSpans, 1)
		w.flushDueToMaxSpansPerPayload()
	}
}

func (w *CollectorTraceWriter) appendTrace(trace *model.Trace) {
	log.Tracef("Handling new trace with %d spans: %v", len(*trace), trace)

	apiTraceChunk := collector.APITraceChunk{
		Spans:    *trace,
		Hostname: w.hostName,
		TraceID:  (*trace)[0].TraceID,
	}
	w.traces = append(w.traces, &apiTraceChunk)
	w.spansInBuffer += len(*trace)
}

func (w *CollectorTraceWriter) flushDueToMaxSpansPerPayload() {
	log.Debugf("Flushing because we reached max per payload")
	w.flush()
}

func (w *CollectorTraceWriter) flush() {
	numTraces := len(w.traces)

	// If no traces, we can't construct anything
	if numTraces == 0 {
		return
	}

	atomic.AddInt64(&w.stats.Traces, int64(numTraces))
	atomic.AddInt64(&w.stats.Spans, int64(w.spansInBuffer))

	sendTracesRequest := collector.SendTracesRequest{
		Chunks: w.traces,
	}

	serialized, err := proto.Marshal(&sendTracesRequest)
	if err != nil {
		log.Errorf("failed to serialize trace payload, data got dropped, err: %s", err)
		w.resetBuffer()
		return
	}

	headers := map[string]string{}

	payload := newPayload(serialized, headers)

	log.Debugf("flushing traces=%v", len(w.traces))
	w.sender.Send(payload)
	w.resetBuffer()
}

func (w *CollectorTraceWriter) resetBuffer() {
	// Reset traces
	w.traces = w.traces[:0]
	w.spansInBuffer = 0
}

func (w *CollectorTraceWriter) updateInfo() {
	var twInfo info.TraceWriterInfo

	// Load counters and reset them for the next flush
	twInfo.Payloads = atomic.SwapInt64(&w.stats.Payloads, 0)
	twInfo.Traces = atomic.SwapInt64(&w.stats.Traces, 0)
	twInfo.Spans = atomic.SwapInt64(&w.stats.Spans, 0)
	twInfo.Bytes = atomic.SwapInt64(&w.stats.Bytes, 0)
	twInfo.Retries = atomic.SwapInt64(&w.stats.Retries, 0)
	twInfo.Errors = atomic.SwapInt64(&w.stats.Errors, 0)
	twInfo.SingleMaxSpans = atomic.SwapInt64(&w.stats.SingleMaxSpans, 0)

	statsd.Client.Count("datadog.trace_agent.collector_trace_writer.payloads", int64(twInfo.Payloads), nil, 1)
	statsd.Client.Count("datadog.trace_agent.collector_trace_writer.traces", int64(twInfo.Traces), nil, 1)
	statsd.Client.Count("datadog.trace_agent.collector_trace_writer.spans", int64(twInfo.Spans), nil, 1)
	statsd.Client.Count("datadog.trace_agent.collector_trace_writer.bytes", int64(twInfo.Bytes), nil, 1)
	statsd.Client.Count("datadog.trace_agent.collector_trace_writer.retries", int64(twInfo.Retries), nil, 1)
	statsd.Client.Count("datadog.trace_agent.collector_trace_writer.errors", int64(twInfo.Errors), nil, 1)
	statsd.Client.Count("datadog.trace_agent.collector_trace_writer.single_max_spans", int64(twInfo.SingleMaxSpans), nil, 1)
}

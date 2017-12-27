package writer

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
	"github.com/golang/protobuf/proto"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// TraceWriter ingests sampled traces and flush them to the API.
type TraceWriter struct {
	endpoint Endpoint

	InTraces       <-chan *model.Trace
	InTransactions <-chan *model.Span

	traceBuffer       []*model.APITrace
	transactionBuffer []*model.Span

	stats info.TraceWriterInfo

	exit   chan struct{}
	exitWG *sync.WaitGroup

	conf *config.AgentConfig
}

// NewTraceWriter returns a new writer for traces.
func NewTraceWriter(conf *config.AgentConfig, InTraces <-chan *model.Trace, InTransactions <-chan *model.Span) *TraceWriter {
	var endpoint Endpoint

	if conf.APIEnabled {
		client := NewClient(conf)
		endpoint = NewDatadogEndpoint(client, conf.APIEndpoint, "/api/v0.2/traces", conf.APIKey)
	} else {
		log.Info("API interface is disabled, flushing to /dev/null instead")
		endpoint = &NullEndpoint{}
	}

	return &TraceWriter{
		endpoint: endpoint,

		traceBuffer:       []*model.APITrace{},
		transactionBuffer: []*model.Span{},

		exit:   make(chan struct{}),
		exitWG: &sync.WaitGroup{},

		InTraces:       InTraces,
		InTransactions: InTransactions,

		conf: conf,
	}
}

// Start starts the writer.
func (w *TraceWriter) Start() {
	go func() {
		defer watchdog.LogOnPanic()
		w.Run()
	}()
}

// Run runs the main loop of the writer goroutine. If buffers payloads and
// services read from input chans and flushes them when necessary.
func (w *TraceWriter) Run() {
	w.exitWG.Add(1)
	defer w.exitWG.Done()

	// for now, simply flush every x seconds
	flushTicker := time.NewTicker(5 * time.Second)
	defer flushTicker.Stop()

	updateInfoTicker := time.NewTicker(1 * time.Minute)
	defer updateInfoTicker.Stop()

	log.Debug("starting trace writer")

	for {
		select {
		case trace := <-w.InTraces:
			// no need for lock for now as flush is sequential
			// TODO: async flush/retry
			apiTrace := trace.APITrace()
			w.traceBuffer = append(w.traceBuffer, apiTrace)
		case transaction := <-w.InTransactions:
			// no need for lock for now as flush is sequential
			// TODO: async flush/retry
			w.transactionBuffer = append(w.transactionBuffer, transaction)
		case <-flushTicker.C:
			w.Flush()
		case <-updateInfoTicker.C:
			go w.updateInfo()
		case <-w.exit:
			log.Info("exiting trace writer, flushing all remaining traces")
			w.Flush()
			return
		}
	}
}

// Stop stops the main Run loop.
func (w *TraceWriter) Stop() {
	close(w.exit)
	w.exitWG.Wait()
}

// Flush flushes traces the data in the API
func (w *TraceWriter) Flush() {
	traces := w.traceBuffer
	transactions := w.transactionBuffer

	if len(traces) == 0 && len(transactions) == 0 {
		log.Debugf("nothing to flush")
		return
	}

	log.Debugf("going to flush %d traces, %d transactions", len(traces), len(transactions))

	atomic.AddInt64(&w.stats.Traces, int64(len(traces)))

	// Make the new buffer of the size of the previous one.
	// that's a fair estimation and it should reduce allocations without using too much memory.
	w.traceBuffer = make([]*model.APITrace, 0, len(traces))
	w.transactionBuffer = make([]*model.Span, 0, len(transactions))

	tracePayload := model.TracePayload{
		HostName:     w.conf.HostName,
		Env:          w.conf.DefaultEnv,
		Traces:       traces,
		Transactions: transactions,
	}

	serialized, err := proto.Marshal(&tracePayload)
	if err != nil {
		log.Errorf("failed to serialize trace payload, data got dropped, err: %s", err)
		return
	}
	atomic.AddInt64(&w.stats.Bytes, int64(len(serialized)))

	// TODO: benchmark and pick the right encoding

	headers := map[string]string{
		languageHeaderKey:  strings.Join(info.Languages(), "|"),
		"Content-Type":     "application/x-protobuf",
		"Content-Encoding": "identity",
	}

	startFlush := time.Now()

	// Send the payload to the endpoint
	err = w.endpoint.Write(serialized, headers)

	flushTime := time.Since(startFlush)

	// TODO: if error, depending on why, replay later.
	if err != nil {
		atomic.AddInt64(&w.stats.Errors, 1)
		log.Errorf("failed to flush trace payload, time:%s, size:%d bytes, error: %s", flushTime, len(serialized), err)
		return
	}

	log.Infof("flushed trace payload to the API, time:%s, size:%d bytes", flushTime, len(serialized))
	statsd.Client.Gauge("datadog.trace_agent.trace_writer.flush_duration", flushTime.Seconds(), nil, 1)
	atomic.AddInt64(&w.stats.Payloads, 1)

}

func (w *TraceWriter) updateInfo() {
	var twInfo info.TraceWriterInfo

	// Load counters and reset them for the next flush
	twInfo.Payloads = atomic.SwapInt64(&w.stats.Payloads, 0)
	twInfo.Traces = atomic.SwapInt64(&w.stats.Traces, 0)
	twInfo.Bytes = atomic.SwapInt64(&w.stats.Bytes, 0)
	twInfo.Errors = atomic.SwapInt64(&w.stats.Traces, 0)

	statsd.Client.Count("datadog.trace_agent.trace_writer.payloads", int64(twInfo.Payloads), nil, 1)
	statsd.Client.Count("datadog.trace_agent.trace_writer.traces", int64(twInfo.Traces), nil, 1)
	statsd.Client.Count("datadog.trace_agent.trace_writer.bytes", int64(twInfo.Bytes), nil, 1)
	statsd.Client.Count("datadog.trace_agent.trace_writer.errors", int64(twInfo.Errors), nil, 1)

	info.UpdateTraceWriterInfo(twInfo)
}

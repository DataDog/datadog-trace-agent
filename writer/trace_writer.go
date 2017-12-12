package writer

import (
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/golang/protobuf/proto"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

const (
	languageHeaderKey = "X-Datadog-Reported-Languages"
)

// TraceWriter ingests sampled traces and flush them to the API.
type TraceWriter struct {
	endpoint Endpoint

	InTraces <-chan *model.Trace

	traceBuffer []*model.APITrace

	exit   chan struct{}
	exitWG *sync.WaitGroup

	conf *config.AgentConfig
}

// NewTraceWriter returns a new writer for traces.
func NewTraceWriter(conf *config.AgentConfig) *TraceWriter {
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

		traceBuffer: []*model.APITrace{},

		exit:   make(chan struct{}),
		exitWG: &sync.WaitGroup{},

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

	for {
		select {
		case trace := <-w.InTraces:
			// no need for lock for now as flush is sequential
			// TODO: async flush/retry
			apiTrace := trace.APITrace()
			w.traceBuffer = append(w.traceBuffer, apiTrace)
		case <-flushTicker.C:
			w.Flush()
		case <-w.exit:
			log.Info("exiting, flushing all remaining traces")
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
	log.Debugf("going to flush %d traces", len(traces))

	// Make the new buffer of the size of the previous one.
	// that's a fair estimation and it should reduce allocations without using too much memory.
	w.traceBuffer = make([]*model.APITrace, 0, len(traces))

	tracePayload := model.TracePayload{
		HostName: w.conf.HostName,
		Env:      w.conf.DefaultEnv,
		Traces:   traces,
	}

	serialized, err := proto.Marshal(&tracePayload)
	if err != nil {
		log.Errorf("failed to serialize trace payload, data got dropped, err: %s", err)
		return
	}

	headers := map[string]string{
		languageHeaderKey: strings.Join(info.Languages(), "|"),
	}

	startFlush := time.Now()

	// Send the payload to the endpoint
	// TODO: track metrics/stats about payload
	err = w.endpoint.Write(serialized, headers)

	flushTime := time.Since(startFlush)

	// TODO: if error, depending on why, replay later.
	if err != nil {
		statsd.Client.Count("datadog.trace_agent.writer.flush", 1, []string{"status:error"}, 1)
		log.Errorf("failed to flush trace payload: %s", err)
	}

	log.Infof("flushed trace payload to the API, time:%s, size:%d bytes", flushTime, len(serialized))
	statsd.Client.Count("datadog.trace_agent.writer.flush", 1, []string{"status:success"}, 1)
	statsd.Client.Gauge("datadog.trace_agent.writer.traces.flush_duration", flushTime.Seconds(), nil, 1)
	statsd.Client.Count("datadog.trace_agent.writer.payload_bytes", int64(len(serialized)), nil, 1)
}

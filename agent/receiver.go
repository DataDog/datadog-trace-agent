package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
	"github.com/tinylib/msgp/msgp"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// The trace agent used to listen on port 7777, but now uses port 8126. Keep
// listening on 7777 during the transition.
const legacyReceiverPort = 7777

const (
	maxRequestBodyLength = 10 * 1024 * 1024
	tagTraceHandler      = "handler:traces"
	tagServiceHandler    = "handler:services"
)

// APIVersion is a dumb way to version our collector handlers
type APIVersion string

const (
	// v01 DEPRECATED, FIXME[1.x]
	// Traces: JSON, slice of spans
	// Services: JSON, map[string]map[string][string]
	v01 APIVersion = "v0.1"
	// v02 DEPRECATED, FIXME[1.x]
	// Traces: JSON, slice of traces
	// Services: JSON, map[string]map[string][string]
	v02 APIVersion = "v0.2"
	// v03
	// Traces: msgpack/JSON (Content-Type) slice of traces
	// Services: msgpack/JSON, map[string]map[string][string]
	v03 APIVersion = "v0.3"
)

// HTTPReceiver is a collector that uses HTTP protocol and just holds
// a chan where the spans received are sent one by one
type HTTPReceiver struct {
	traces   chan model.Trace
	services chan model.ServicesMetadata
	conf     *config.AgentConfig

	// due to the high volume the receiver handles
	// custom logger that rate-limits errors and track statistics
	logger     *errorLogger
	stats      receiverStats
	preSampler *sampler.PreSampler
	exit       chan struct{}

	maxRequestBodyLength int64
	debug                bool
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver(conf *config.AgentConfig) *HTTPReceiver {
	// use buffered channels so that handlers are not waiting on downstream processing
	logger := &errorLogger{}
	return &HTTPReceiver{
		traces:     make(chan model.Trace, 5000), // about 1000 traces/sec for 5 sec
		services:   make(chan model.ServicesMetadata, 50),
		conf:       conf,
		logger:     logger,
		preSampler: sampler.NewPreSampler(conf.PreSampleRate, logger),
		exit:       make(chan struct{}),

		maxRequestBodyLength: maxRequestBodyLength,
		debug:                strings.ToLower(conf.LogLevel) == "debug",
	}
}

// Run starts doing the HTTP server and is ready to receive traces
func (r *HTTPReceiver) Run() {
	// FIXME[1.x]: remove all those legacy endpoints + code that goes with it
	http.HandleFunc("/spans", r.httpHandleWithVersion(v01, r.handleTraces))
	http.HandleFunc("/services", r.httpHandleWithVersion(v01, r.handleServices))
	http.HandleFunc("/v0.1/spans", r.httpHandleWithVersion(v01, r.handleTraces))
	http.HandleFunc("/v0.1/services", r.httpHandleWithVersion(v01, r.handleServices))
	http.HandleFunc("/v0.2/traces", r.httpHandleWithVersion(v02, r.handleTraces))
	http.HandleFunc("/v0.2/services", r.httpHandleWithVersion(v02, r.handleServices))

	// current collector API
	http.HandleFunc("/v0.3/traces", r.httpHandleWithVersion(v03, r.handleTraces))
	http.HandleFunc("/v0.3/services", r.httpHandleWithVersion(v03, r.handleServices))

	// expvar implicitely publishes "/debug/vars" on the same port

	addr := fmt.Sprintf("%s:%d", r.conf.ReceiverHost, r.conf.ReceiverPort)
	if err := r.Listen(addr, ""); err != nil {
		die("%v", err)
	}

	legacyAddr := fmt.Sprintf("%s:%d", r.conf.ReceiverHost, legacyReceiverPort)
	if err := r.Listen(legacyAddr, " (legacy)"); err != nil {
		log.Error(err)
	}

	watchdog.Go(func() {
		r.logStats()
	})
}

// Listen creates a new HTTP server listening on the provided address.
func (r *HTTPReceiver) Listen(addr, logExtra string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %v", addr, err)
	}

	stoppableListener, err := NewStoppableListener(listener, r.exit,
		r.conf.ConnectionLimit)
	if err != nil {
		return fmt.Errorf("cannot create stoppable listener: %v", err)
	}

	timeout := 5 * time.Second
	if r.conf.ReceiverTimeout > 0 {
		timeout = time.Duration(r.conf.ReceiverTimeout) * time.Second
	}

	server := http.Server{
		ReadTimeout:  time.Second * time.Duration(timeout),
		WriteTimeout: time.Second * time.Duration(timeout),
	}

	log.Infof("listening for traces at http://%s%s", addr, logExtra)

	watchdog.Go(func() {
		stoppableListener.Refresh(r.conf.ConnectionLimit)
	})
	watchdog.Go(func() {
		server.Serve(stoppableListener)
	})

	return nil
}

func (r *HTTPReceiver) httpHandle(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		req.Body = model.NewLimitedReader(req.Body, r.maxRequestBodyLength)
		defer req.Body.Close()

		fn(w, req)
	}
}

func (r *HTTPReceiver) httpHandleWithVersion(v APIVersion, f func(APIVersion, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return r.httpHandle(func(w http.ResponseWriter, req *http.Request) {
		contentType := req.Header.Get("Content-Type")
		if contentType == "application/msgpack" && (v == v01 || v == v02) {
			// msgpack is only supported for versions 0.3
			r.logger.Errorf("rejecting client request, unsupported media type %q", contentType)
			HTTPFormatError([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}

		f(v, w, req)
	})
}

// handleTraces knows how to handle a bunch of traces
func (r *HTTPReceiver) handleTraces(v APIVersion, w http.ResponseWriter, req *http.Request) {
	if !r.preSampler.Sample(req) {
		// [TODO:christian] keep a trace of this, update weights accordingly
		HTTPOK(w)
		return
	}

	preRate := r.preSampler.Rate()

	var traces model.Traces
	contentType := req.Header.Get("Content-Type")

	switch v {
	case v01:
		// We cannot use decodeReceiverPayload because []model.Span does not
		// implement msgp.Decodable. This hack can be removed once we
		// drop v01 support.
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			r.logger.Errorf("rejecting client request, unsupported media type %q", contentType)
			HTTPFormatError([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}

		// in v01 we actually get spans that we have to transform in traces
		var spans []model.Span
		if err := json.NewDecoder(req.Body).Decode(&spans); err != nil {
			r.logger.Errorf("cannot decode %s traces payload: %v", v, err)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}
		traces = model.TracesFromSpans(spans)

	case v02:
		fallthrough
	case v03:
		if err := decodeReceiverPayload(req.Body, &traces, v, contentType); err != nil {
			r.logger.Errorf("cannot decode %s traces payload: %v", v, err)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}

	default:
		HTTPEndpointNotSupported([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
		return
	}

	HTTPOK(w)

	bytesRead := req.Body.(*model.LimitedReader).Count
	if bytesRead > 0 {
		atomic.AddInt64(&r.stats.TracesBytes, int64(bytesRead))
	}

	// normalize data
	for i := range traces {
		traces[i].ApplyRate(preRate)

		spans := len(traces[i])
		normTrace, err := model.NormalizeTrace(traces[i])
		if err != nil {
			atomic.AddInt64(&r.stats.TracesDropped, 1)
			atomic.AddInt64(&r.stats.SpansDropped, int64(spans))

			errorMsg := fmt.Sprintf("dropping trace reason: %s (debug for more info), %v", err, normTrace)
			if len(errorMsg) > 150 && r.debug {
				errorMsg = errorMsg[:150] + "..."
			}
			r.logger.Errorf(errorMsg)
		} else {
			atomic.AddInt64(&r.stats.SpansDropped, int64(spans-len(normTrace)))

			// if our downstream consumer is slow, we drop the trace on the floor
			// this is a safety net against us using too much memory
			// when clients flood us
			select {
			case r.traces <- normTrace:
			default:
				atomic.AddInt64(&r.stats.TracesDropped, 1)
				atomic.AddInt64(&r.stats.SpansDropped, int64(spans))

				r.logger.Errorf("dropping trace reason: rate-limited")
			}
		}

		atomic.AddInt64(&r.stats.TracesReceived, 1)
		atomic.AddInt64(&r.stats.SpansReceived, int64(spans))
	}
}

// handleServices handle a request with a list of several services
func (r *HTTPReceiver) handleServices(v APIVersion, w http.ResponseWriter, req *http.Request) {

	var servicesMeta model.ServicesMetadata

	contentType := req.Header.Get("Content-Type")
	if err := decodeReceiverPayload(req.Body, &servicesMeta, v, contentType); err != nil {
		r.logger.Errorf("cannot decode %s services payload: %v", v, err)
		HTTPDecodingError(err, []string{tagServiceHandler, fmt.Sprintf("v:%s", v)}, w)
		return
	}

	statsd.Client.Count("datadog.trace_agent.receiver.service", int64(len(servicesMeta)), nil, 1)
	HTTPOK(w)

	bytesRead := req.Body.(*model.LimitedReader).Count
	if bytesRead > 0 {
		atomic.AddInt64(&r.stats.TracesBytes, int64(bytesRead))
	}

	r.services <- servicesMeta
}

// logStats periodically submits stats about the receiver to statsd
func (r *HTTPReceiver) logStats() {
	var accStats receiverStats
	var lastLog time.Time

	for now := range time.Tick(10 * time.Second) {
		// Load counters and reset them for the next flush
		tracesBytes := atomic.SwapInt64(&r.stats.TracesBytes, 0)
		accStats.TracesBytes += tracesBytes

		servicesBytes := atomic.SwapInt64(&r.stats.ServicesBytes, 0)
		accStats.ServicesBytes += servicesBytes

		spans := atomic.SwapInt64(&r.stats.SpansReceived, 0)
		accStats.SpansReceived += spans

		traces := atomic.SwapInt64(&r.stats.TracesReceived, 0)
		accStats.TracesReceived += traces

		sdropped := atomic.SwapInt64(&r.stats.SpansDropped, 0)
		accStats.SpansDropped += sdropped

		tdropped := atomic.SwapInt64(&r.stats.TracesDropped, 0)
		accStats.TracesDropped += tdropped

		statsd.Client.Gauge("datadog.trace_agent.heartbeat", 1, []string{fmt.Sprintf("version:%s", Version)}, 1)

		statsd.Client.Count("datadog.trace_agent.receiver.traces", tracesBytes, []string{"endpoint:traces"}, 1)
		statsd.Client.Count("datadog.trace_agent.receiver.services", servicesBytes, []string{"endpoint:services"}, 1)
		statsd.Client.Count("datadog.trace_agent.receiver.span", spans, nil, 1)
		statsd.Client.Count("datadog.trace_agent.receiver.trace", traces, nil, 1)
		statsd.Client.Count("datadog.trace_agent.receiver.span_dropped", sdropped, nil, 1)
		statsd.Client.Count("datadog.trace_agent.receiver.trace_dropped", tdropped, nil, 1)

		if now.Sub(lastLog) >= time.Minute {
			updateReceiverStats(accStats)
			log.Infof("receiver handled %d spans, dropped %d ; handled %d traces, dropped %d",
				accStats.SpansReceived, accStats.SpansDropped,
				accStats.TracesReceived, accStats.TracesDropped)
			r.logger.Reset()

			accStats = receiverStats{}
			lastLog = now
		}
	}
}

// receiverStats contains stats about the volume of data received
type receiverStats struct {
	// TracesBytes is the amount of data received on the traces endpoint (raw data, encoded, compressed).
	TracesBytes int64
	// ServicesBytes is the amount of data received on the services endpoint (raw data, encoded, compressed).
	ServicesBytes int64
	// SpansReceived is the number of spans received, including the dropped ones
	SpansReceived int64
	// TracesReceived is the number of traces received, including the dropped ones
	TracesReceived int64
	// SpansDropped is the number of spans dropped
	SpansDropped int64
	// SpansReceived is the number of traces dropped
	TracesDropped int64
}

func decodeReceiverPayload(r io.Reader, dest msgp.Decodable, v APIVersion, contentType string) error {
	switch contentType {
	case "application/msgpack":
		return msgp.Decode(r, dest)

	case "application/json":
		fallthrough
	case "text/json":
		fallthrough
	case "":
		return json.NewDecoder(r).Decode(dest)

	default:
		panic(fmt.Sprintf("unhandled content type %q", contentType))
	}
}

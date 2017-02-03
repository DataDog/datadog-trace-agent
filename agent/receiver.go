package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	log "github.com/cihub/seelog"
)

// The trace agent used to listen on port 7777, but now uses port 8126. Keep
// listening on 7777 during the transition.
const legacyReceiverPort = 7777

// APIVersion is a dumb way to version our collector handlers
type APIVersion int

const (
	decoderSize       = 10 // Max size of decoders pool
	tagTraceHandler   = "handler:traces"
	tagServiceHandler = "handler:services"
)

const (
	// v01 DEPRECATED, FIXME[1.x]
	// Traces: JSON, slice of spans
	// Services: JSON, map[string]map[string][string]
	v01 APIVersion = iota
	// v02 DEPRECATED, FIXME[1.x]
	// Traces: JSON, slice of traces
	// Services: JSON, map[string]map[string][string]
	v02
	// v03
	// Traces: msgpack/JSON (Content-Type) slice of traces
	// Services: msgpack/JSON, map[string]map[string][string]
	v03
)

func httpHandleWithVersion(v APIVersion, f func(APIVersion, http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		f(v, w, r)
	}
}

// HTTPReceiver is a collector that uses HTTP protocol and just holds
// a chan where the spans received are sent one by one
type HTTPReceiver struct {
	traces      chan model.Trace
	services    chan model.ServicesMetadata
	decoderPool *model.DecoderPool
	conf        *config.AgentConfig

	// due to the high volume the receiver handles
	// custom logger that rate-limits errors and track statistics
	logger *errorLogger
	stats  receiverStats

	exit  chan struct{}
	debug bool
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver(conf *config.AgentConfig) *HTTPReceiver {
	// use buffered channels so that handlers are not waiting on downstream processing
	return &HTTPReceiver{
		traces:      make(chan model.Trace, 50),
		services:    make(chan model.ServicesMetadata, 50),
		decoderPool: model.NewDecoderPool(decoderSize),
		conf:        conf,
		logger:      &errorLogger{},
		exit:        make(chan struct{}),
		debug:       strings.ToLower(conf.LogLevel) == "debug",
	}
}

// Run starts doing the HTTP server and is ready to receive traces
func (r *HTTPReceiver) Run() {
	// FIXME[1.x]: remove all those legacy endpoints + code that goes with it
	http.HandleFunc("/spans", httpHandleWithVersion(v01, r.handleTraces))
	http.HandleFunc("/services", httpHandleWithVersion(v01, r.handleServices))
	http.HandleFunc("/v0.1/spans", httpHandleWithVersion(v01, r.handleTraces))
	http.HandleFunc("/v0.1/services", httpHandleWithVersion(v01, r.handleServices))
	http.HandleFunc("/v0.2/traces", httpHandleWithVersion(v02, r.handleTraces))
	http.HandleFunc("/v0.2/services", httpHandleWithVersion(v02, r.handleServices))

	// current collector API
	http.HandleFunc("/v0.3/traces", httpHandleWithVersion(v03, r.handleTraces))
	http.HandleFunc("/v0.3/services", httpHandleWithVersion(v03, r.handleServices))

	// expvar implicitely publishes "/debug/vars" on the same port

	addr := fmt.Sprintf("%s:%d", r.conf.ReceiverHost, r.conf.ReceiverPort)
	if err := r.Listen(addr, ""); err != nil {
		die("%v", err)
	}

	legacyAddr := fmt.Sprintf("%s:%d", r.conf.ReceiverHost, legacyReceiverPort)
	if err := r.Listen(legacyAddr, " (legacy)"); err != nil {
		log.Error(err)
	}

	go r.logStats()
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

	go stoppableListener.Refresh(r.conf.ConnectionLimit)
	go server.Serve(stoppableListener)

	return nil
}

// handleTraces knows how to handle a bunch of traces
func (r *HTTPReceiver) handleTraces(v APIVersion, w http.ResponseWriter, req *http.Request) {
	// we need an io.ReadSeeker if we want to be able to display
	// error feedback to the user, otherwise r.Body is trash
	// once it's been decoded
	if req.Body == nil {
		return
	}
	defer req.Body.Close()

	var traces model.Traces
	contentType := req.Header.Get("Content-Type")

	switch v {
	case v01:
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			r.logger.Errorf("rejecting client request, unsupported media type: '%s'", contentType)
			HTTPFormatError([]string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}

		// in v01 we actually get spans that we have to transform in traces
		var spans []model.Span
		dec := r.decoderPool.Borrow(contentType)
		err := dec.Decode(req.Body, &spans)
		if err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			r.decoderPool.Release(dec)
			HTTPDecodingError([]string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}

		r.decoderPool.Release(dec)
		traces = model.TracesFromSpans(spans)
	case v02:
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			r.logger.Errorf("rejecting client request, unsupported media type: '%s'", contentType)
			HTTPFormatError([]string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}

		dec := r.decoderPool.Borrow(contentType)
		err := dec.Decode(req.Body, &traces)
		if err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			r.decoderPool.Release(dec)
			HTTPDecodingError([]string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}

		r.decoderPool.Release(dec)
	case v03:
		// select the right Decoder based on the given content-type header
		dec := r.decoderPool.Borrow(contentType)
		err := dec.Decode(req.Body, &traces)
		if err != nil {
			if strings.Contains(contentType, "json") {
				r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			} else {
				r.logger.Errorf("error when decoding msgpack traces")
			}
			r.decoderPool.Release(dec)
			HTTPDecodingError([]string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}

		r.decoderPool.Release(dec)
	default:
		HTTPEndpointNotSupported([]string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
		return
	}

	HTTPOK(w)

	// normalize data
	for i := range traces {
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
			r.traces <- normTrace
		}

		atomic.AddInt64(&r.stats.TracesReceived, 1)
		atomic.AddInt64(&r.stats.SpansReceived, int64(spans))
	}
}

// handleServices handle a request with a list of several services
func (r *HTTPReceiver) handleServices(v APIVersion, w http.ResponseWriter, req *http.Request) {

	// we need an io.ReadSeeker if we want to be able to display
	// error feedback to the user, otherwise req.Body is trash
	// once it's been decoded
	if req.Body == nil {
		return
	}
	defer req.Body.Close()

	var servicesMeta model.ServicesMetadata
	contentType := req.Header.Get("Content-Type")

	switch v {
	case v01:
		fallthrough
	case v02:
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			r.logger.Errorf("rejecting client request, unsupported media type: '%s'", contentType)
			HTTPFormatError([]string{tagServiceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}

		// select the right Decoder based on the given content-type header
		dec := r.decoderPool.Borrow(contentType)
		err := dec.Decode(req.Body, &servicesMeta)
		if err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			HTTPDecodingError([]string{tagServiceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}
	case v03:
		// select the right Decoder based on the given content-type header
		dec := r.decoderPool.Borrow(contentType)
		err := dec.Decode(req.Body, &servicesMeta)
		if err != nil {
			if strings.Contains(contentType, "json") {
				r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			} else {
				r.logger.Errorf("error when decoding msgpack traces")
			}
			HTTPDecodingError([]string{tagServiceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}
	default:
		HTTPEndpointNotSupported([]string{tagServiceHandler, fmt.Sprintf("v:%d", v)}, w)
		return
	}

	statsd.Client.Count("datadog.trace_agent.receiver.service", int64(len(servicesMeta)), nil, 1)
	HTTPOK(w)

	r.services <- servicesMeta
}

// logStats periodically submits stats about the receiver to statsd
func (r *HTTPReceiver) logStats() {
	var accStats receiverStats
	var lastLog time.Time

	for now := range time.Tick(60 * time.Second) {
		// Load counters and reset them for the next flush
		spans := atomic.LoadInt64(&r.stats.SpansReceived)
		r.stats.SpansReceived = 0

		traces := atomic.LoadInt64(&r.stats.TracesReceived)
		r.stats.TracesReceived = 0

		sdropped := atomic.LoadInt64(&r.stats.SpansDropped)
		r.stats.SpansDropped = 0

		tdropped := atomic.LoadInt64(&r.stats.TracesDropped)
		r.stats.TracesDropped = 0

		statsd.Client.Gauge("datadog.trace_agent.heartbeat", 1, []string{fmt.Sprintf("version:%s", Version)}, 1)

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
	// SpansReceived is the number of spans received, including the dropped ones
	SpansReceived int64
	// TracesReceived is the number of traces received, including the dropped ones
	TracesReceived int64
	// SpansDropped is the number of spans dropped
	SpansDropped int64
	// SpansReceived is the number of traces dropped
	TracesDropped int64
}

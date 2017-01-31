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
	decoderSize          = 10 // Max size of decoders pool
	maxRequestBodyLength = 10 * 1024 * 1024
	tagTraceHandler      = "handler:traces"
	tagServiceHandler    = "handler:services"
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

	exit chan struct{}

	maxRequestBodyLength int64
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver(conf *config.AgentConfig) *HTTPReceiver {
	// use buffered channels so that handlers are not waiting on downstream processing
	return &HTTPReceiver{
		traces:      make(chan model.Trace, 5000), // about 1000 traces/sec for 5 sec
		services:    make(chan model.ServicesMetadata, 50),
		decoderPool: model.NewDecoderPool(decoderSize),
		conf:        conf,
		logger:      &errorLogger{},
		exit:        make(chan struct{}),

		maxRequestBodyLength: maxRequestBodyLength,
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

	addr := fmt.Sprintf("%s:%d", r.conf.ReceiverHost, r.conf.ReceiverPort)
	if err := r.Listen(addr); err != nil {
		die("%v", err)
	}

	legacyAddr := fmt.Sprintf("%s:%d", r.conf.ReceiverHost, legacyReceiverPort)
	if err := r.Listen(legacyAddr); err != nil {
		log.Error(err)
	}

	go r.logStats()
}

// Listen creates a new HTTP server listening on the provided address.
func (r *HTTPReceiver) Listen(addr string) error {
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

	log.Infof("listening for traces at http://%s", addr)

	go stoppableListener.Refresh(r.conf.ConnectionLimit)
	go server.Serve(stoppableListener)

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
		f(v, w, req)
	})
}

// handleTraces knows how to handle a bunch of traces
func (r *HTTPReceiver) handleTraces(v APIVersion, w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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
		if err := dec.Decode(req.Body, &spans); err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			r.decoderPool.Release(dec)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
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
		if err := dec.Decode(req.Body, &traces); err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			r.decoderPool.Release(dec)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}

		r.decoderPool.Release(dec)
	case v03:
		// select the right Decoder based on the given content-type header
		dec := r.decoderPool.Borrow(contentType)
		if err := dec.Decode(req.Body, &traces); err != nil {
			if strings.Contains(contentType, "json") {
				r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			} else {
				r.logger.Errorf("error when decoding msgpack traces")
			}
			r.decoderPool.Release(dec)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%d", v)}, w)
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

			// this is a potentially very spammy log message, so extra care
			errorMsg := fmt.Sprintf("dropping trace reason: %s (debug for more info), %v", err, normTrace)
			if len(errorMsg) > 150 {
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
	if req.Method != "POST" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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
		if err := dec.Decode(req.Body, &servicesMeta); err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			HTTPDecodingError(err, []string{tagServiceHandler, fmt.Sprintf("v:%d", v)}, w)
			return
		}
	case v03:
		// select the right Decoder based on the given content-type header
		dec := r.decoderPool.Borrow(contentType)
		if err := dec.Decode(req.Body, &servicesMeta); err != nil {
			if strings.Contains(contentType, "json") {
				r.logger.Errorf(model.HumanReadableJSONError(dec.BufferReader(), err))
			} else {
				r.logger.Errorf("error when decoding msgpack traces")
			}
			HTTPDecodingError(err, []string{tagServiceHandler, fmt.Sprintf("v:%d", v)}, w)
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

	for now := range time.Tick(10 * time.Second) {
		// Load counters and reset them for the next flush
		spans := atomic.SwapInt64(&r.stats.SpansReceived, 0)
		accStats.SpansReceived += spans

		traces := atomic.SwapInt64(&r.stats.TracesReceived, 0)
		accStats.TracesReceived += traces

		sdropped := atomic.SwapInt64(&r.stats.SpansDropped, 0)
		accStats.SpansDropped += sdropped

		tdropped := atomic.SwapInt64(&r.stats.TracesDropped, 0)
		accStats.TracesDropped += tdropped

		statsd.Client.Gauge("datadog.trace_agent.heartbeat", 1, []string{fmt.Sprintf("version:%s", Version)}, 1)

		statsd.Client.Count("datadog.trace_agent.receiver.span", spans, nil, 1)
		statsd.Client.Count("datadog.trace_agent.receiver.trace", traces, nil, 1)
		statsd.Client.Count("datadog.trace_agent.receiver.span_dropped", sdropped, nil, 1)
		statsd.Client.Count("datadog.trace_agent.receiver.trace_dropped", tdropped, nil, 1)

		if now.Sub(lastLog) >= 60*time.Second {
			log.Infof("receiver handled %d spans, dropped %d ; handled %d traces, dropped %d",
				accStats.SpansReceived, accStats.SpansDropped,
				accStats.TracesReceived, accStats.TracesDropped)
			r.logger.Reset()

			accStats = receiverStats{}
			lastLog = now
		}
	}
}

type receiverStats struct {
	SpansReceived  int64
	TracesReceived int64
	SpansDropped   int64
	TracesDropped  int64
}

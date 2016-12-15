package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/statsd"
	log "github.com/cihub/seelog"
)

// APIVersion is a dumb way to version our collector handlers
type APIVersion int

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
	traces   chan model.Trace
	services chan model.ServicesMetadata
	conf     *config.AgentConfig

	// due to the high volume the receiver handles
	// custom logger that rate-limits errors and track statistics
	logger *errorLogger
	stats  receiverStats

	exit chan struct{}
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver(conf *config.AgentConfig) *HTTPReceiver {
	// use buffered channels so that handlers are not waiting on downstream processing
	return &HTTPReceiver{
		traces:   make(chan model.Trace, 50),
		services: make(chan model.ServicesMetadata, 50),
		conf:     conf,
		logger:   &errorLogger{},
		exit:     make(chan struct{}),
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

	addr := fmt.Sprintf("%s:%d", r.conf.ReceiverHost, r.conf.ReceiverPort)
	log.Infof("listening for traces at http://%s/", addr)

	tcpL, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("could not create TCP listener")
		panic(err)
	}

	sl, err := NewStoppableListener(tcpL, r.exit, r.conf.ConnectionLimit)
	// some clients might use keep-alive and keep open their connections too long
	// avoid leaks
	server := http.Server{ReadTimeout: 5 * time.Second}

	go r.logStats()
	go sl.Refresh(r.conf.ConnectionLimit)
	go server.Serve(sl)
}

// handleTraces knows how to handle a bunch of traces
func (r *HTTPReceiver) handleTraces(v APIVersion, w http.ResponseWriter, req *http.Request) {
	handlerTags := []string{"handler:traces", fmt.Sprintf("v:%d", v)}
	// we need an io.ReadSeeker if we want to be able to display
	// error feedback to the user, otherwise r.Body is trash
	// once it's been decoded
	if req.Body == nil {
		return
	}
	defer req.Body.Close()
	bodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return
	}

	bodyBuffer := bytes.NewReader(bodyBytes)
	contentType := req.Header.Get("Content-Type")

	var traces []model.Trace

	switch v {
	case v01:
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			r.logger.Errorf("rejecting client request, unsupported media type: '%s'", contentType)
			HTTPFormatError(handlerTags, w)
			return
		}

		// in v01 we actually get spans that we have to transform in traces
		var spans []model.Span
		dec := json.NewDecoder(bodyBuffer)
		err := dec.Decode(&spans)
		if err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(bodyBuffer, err))
			HTTPDecodingError(handlerTags, w)
			return
		}

		traces = model.TracesFromSpans(spans)
	case v02:
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			r.logger.Errorf("rejecting client request, unsupported media type: '%s'", contentType)
			HTTPFormatError(handlerTags, w)
			return
		}

		dec := json.NewDecoder(bodyBuffer)
		err := dec.Decode(&traces)
		if err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(bodyBuffer, err))
			HTTPDecodingError(handlerTags, w)
			return
		}
	case v03:
		// select the right Decoder based on the given content-type header
		dec := model.DecoderFromContentType(contentType, bodyBuffer)
		err := dec.Decode(&traces)
		if err != nil {
			if strings.Contains(contentType, "json") {
				r.logger.Errorf(model.HumanReadableJSONError(bodyBuffer, err))
			} else {
				r.logger.Errorf("error when decoding msgpack traces")
			}
			HTTPDecodingError(handlerTags, w)
			return
		}
	default:
		HTTPEndpointNotSupported(handlerTags, w)
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
			r.traces <- normTrace
		}

		atomic.AddInt64(&r.stats.TracesReceived, 1)
		atomic.AddInt64(&r.stats.SpansReceived, int64(spans))
	}
}

// handleServices handle a request with a list of several services
func (r *HTTPReceiver) handleServices(v APIVersion, w http.ResponseWriter, req *http.Request) {
	handlerTags := []string{"handler:services", fmt.Sprintf("v:%d", v)}

	// we need an io.ReadSeeker if we want to be able to display
	// error feedback to the user, otherwise req.Body is trash
	// once it's been decoded
	if req.Body == nil {
		return
	}

	defer req.Body.Close()
	bodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return
	}

	var servicesMeta model.ServicesMetadata
	bodyBuffer := bytes.NewReader(bodyBytes)
	contentType := req.Header.Get("Content-Type")

	switch v {
	case v01:
		fallthrough
	case v02:
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			r.logger.Errorf("rejecting client request, unsupported media type: '%s'", contentType)
			HTTPFormatError(handlerTags, w)
			return
		}

		dec := json.NewDecoder(bodyBuffer)
		err = dec.Decode(&servicesMeta)
		if err != nil {
			r.logger.Errorf(model.HumanReadableJSONError(bodyBuffer, err))
			HTTPDecodingError(handlerTags, w)
			return
		}
	case v03:
		// select the right Decoder based on the given content-type header
		dec := model.DecoderFromContentType(contentType, bodyBuffer)
		err = dec.Decode(&servicesMeta)
		if err != nil {
			if strings.Contains(contentType, "json") {
				r.logger.Errorf(model.HumanReadableJSONError(bodyBuffer, err))
			} else {
				r.logger.Errorf("error when decoding msgpack traces")
			}
			HTTPDecodingError(handlerTags, w)
			return
		}
	default:
		HTTPEndpointNotSupported(handlerTags, w)
		return
	}

	statsd.Client.Count("trace_agent.receiver.service", int64(len(servicesMeta)), nil, 1)
	HTTPOK(w)

	r.services <- servicesMeta
}

// logStats periodically submits stats about the receiver to statsd
func (r *HTTPReceiver) logStats() {
	for range time.Tick(60 * time.Second) {
		// Load counters and reset them for the next flush
		spans := atomic.LoadInt64(&r.stats.SpansReceived)
		r.stats.SpansReceived = 0

		traces := atomic.LoadInt64(&r.stats.TracesReceived)
		r.stats.TracesReceived = 0

		sdropped := atomic.LoadInt64(&r.stats.SpansDropped)
		r.stats.SpansDropped = 0

		tdropped := atomic.LoadInt64(&r.stats.TracesDropped)
		r.stats.TracesDropped = 0

		statsd.Client.Count("trace_agent.receiver.span", spans, nil, 1)
		statsd.Client.Count("trace_agent.receiver.trace", traces, nil, 1)
		statsd.Client.Count("trace_agent.receiver.span_dropped", sdropped, nil, 1)
		statsd.Client.Count("trace_agent.receiver.trace_dropped", tdropped, nil, 1)

		log.Infof("receiver handled %d spans, dropped %d ; handled %d traces, dropped %d", spans, sdropped, traces, tdropped)
		r.logger.Reset()
	}
}

type receiverStats struct {
	SpansReceived  int64
	TracesReceived int64
	SpansDropped   int64
	TracesDropped  int64
}

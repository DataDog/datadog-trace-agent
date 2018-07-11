package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
	"github.com/tinylib/msgp/msgp"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/osutil"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

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
	// v04
	// Traces: msgpack/JSON (Content-Type) slice of traces + returns service sampling ratios
	// Services: msgpack/JSON, map[string]map[string][string]
	v04 APIVersion = "v0.4"
)

// HTTPReceiver is a collector that uses HTTP protocol and just holds
// a chan where the spans received are sent one by one
type HTTPReceiver struct {
	traces   chan model.Trace
	services chan model.ServicesMetadata
	conf     *config.AgentConfig
	dynConf  *config.DynamicConfig
	server   *http.Server

	stats      *info.ReceiverStats
	preSampler *sampler.PreSampler

	maxRequestBodyLength int64
	debug                bool
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver(
	conf *config.AgentConfig, dynConf *config.DynamicConfig, traces chan model.Trace, services chan model.ServicesMetadata,
) *HTTPReceiver {
	// use buffered channels so that handlers are not waiting on downstream processing
	return &HTTPReceiver{
		conf:       conf,
		dynConf:    dynConf,
		stats:      info.NewReceiverStats(),
		preSampler: sampler.NewPreSampler(conf.PreSampleRate),

		traces:   traces,
		services: services,

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
	http.HandleFunc("/v0.3/traces", r.httpHandleWithVersion(v03, r.handleTraces))
	http.HandleFunc("/v0.3/services", r.httpHandleWithVersion(v03, r.handleServices))

	// current collector API
	http.HandleFunc("/v0.4/traces", r.httpHandleWithVersion(v04, r.handleTraces))
	http.HandleFunc("/v0.4/services", r.httpHandleWithVersion(v04, r.handleServices))

	// Zipkin v2 endpoint
	// TODO(gbbr): disable by default, add yaml flag
	http.HandleFunc("/zipkin/v2/spans", r.httpHandle(r.handleZipkinSpans))

	// expvar implicitely publishes "/debug/vars" on the same port
	addr := fmt.Sprintf("%s:%d", r.conf.ReceiverHost, r.conf.ReceiverPort)
	if err := r.Listen(addr, ""); err != nil {
		osutil.Exitf("%v", err)
	}

	go r.preSampler.Run()

	go func() {
		defer watchdog.LogOnPanic()
		r.logStats()
	}()
}

// Listen creates a new HTTP server listening on the provided address.
func (r *HTTPReceiver) Listen(addr, logExtra string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %v", addr, err)
	}

	ln, err := NewRateLimitedListener(listener, r.conf.ConnectionLimit)
	if err != nil {
		return fmt.Errorf("cannot create listener: %v", err)
	}
	timeout := 5 * time.Second
	if r.conf.ReceiverTimeout > 0 {
		timeout = time.Duration(r.conf.ReceiverTimeout) * time.Second
	}
	r.server = &http.Server{
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}
	log.Infof("listening for traces at http://%s%s", addr, logExtra)

	go func() {
		defer watchdog.LogOnPanic()
		ln.Refresh(r.conf.ConnectionLimit)
	}()
	go func() {
		defer watchdog.LogOnPanic()
		r.server.Serve(ln)
	}()

	return nil
}

// Stop stops the receiver and shuts down the HTTP server.
func (r *HTTPReceiver) Stop() error {
	expiry := time.Now().Add(20 * time.Second) // give it 20 seconds
	ctx, _ := context.WithDeadline(context.Background(), expiry)
	return r.server.Shutdown(ctx)
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
			log.Errorf("rejecting client request, unsupported media type %q", contentType)
			HTTPFormatError([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}

		f(v, w, req)
	})
}

func (r *HTTPReceiver) replyTraces(v APIVersion, w http.ResponseWriter) {
	switch v {
	case v01:
		fallthrough
	case v02:
		fallthrough
	case v03:
		// Simple response, simply acknowledge with "OK"
		HTTPOK(w)
	case v04:
		// Return the recommended sampling rate for each service as a JSON.
		HTTPRateByService(w, r.dynConf)
	}
}

// handleTraces knows how to handle a bunch of traces
func (r *HTTPReceiver) handleTraces(v APIVersion, w http.ResponseWriter, req *http.Request) {
	if !r.preSampler.Sample(req) {
		io.Copy(ioutil.Discard, req.Body)
		HTTPOK(w)
		return
	}

	traces, ok := getTraces(v, w, req)
	if !ok {
		return
	}

	// We successfuly decoded the payload
	r.replyTraces(v, w)

	// We parse the tags from the header
	tags := info.Tags{
		Lang:          req.Header.Get("Datadog-Meta-Lang"),
		LangVersion:   req.Header.Get("Datadog-Meta-Lang-Version"),
		Interpreter:   req.Header.Get("Datadog-Meta-Lang-Interpreter"),
		TracerVersion: req.Header.Get("Datadog-Meta-Tracer-Version"),
	}
	var size int64
	lr, ok := req.Body.(*model.LimitedReader)
	if ok {
		size = lr.Count
	}
	r.receiveTraces(traces, tags, size)
}

func (r *HTTPReceiver) receiveTraces(traces []model.Trace, tags info.Tags, bytesRead int64) {
	// We get the address of the struct holding the stats associated to the tags
	ts := r.stats.GetTagStats(tags)
	if bytesRead > 0 {
		atomic.AddInt64(&ts.TracesBytes, int64(bytesRead))
	}
	// normalize data
	for i := range traces {
		spans := len(traces[i])

		atomic.AddInt64(&ts.TracesReceived, 1)
		atomic.AddInt64(&ts.SpansReceived, int64(spans))

		normTrace, err := model.NormalizeTrace(traces[i])
		if err != nil {
			atomic.AddInt64(&ts.TracesDropped, 1)
			atomic.AddInt64(&ts.SpansDropped, int64(spans))

			errorMsg := fmt.Sprintf("dropping trace reason: %s (debug for more info), %v", err, normTrace)

			// avoid truncation in DEBUG mode
			if len(errorMsg) > 150 && !r.debug {
				errorMsg = errorMsg[:150] + "..."
			}
			log.Errorf(errorMsg)
		} else {
			atomic.AddInt64(&ts.SpansDropped, int64(spans-len(normTrace)))

			select {
			case r.traces <- normTrace:
				// if our downstream consumer is slow, we drop the trace on the floor
				// this is a safety net against us using too much memory
				// when clients flood us
			default:
				atomic.AddInt64(&ts.TracesDropped, 1)
				atomic.AddInt64(&ts.SpansDropped, int64(spans))

				log.Errorf("dropping trace reason: rate-limited")
			}
		}
	}
}

// handleServices handle a request with a list of several services
func (r *HTTPReceiver) handleServices(v APIVersion, w http.ResponseWriter, req *http.Request) {
	var servicesMeta model.ServicesMetadata

	contentType := req.Header.Get("Content-Type")
	if err := decodeReceiverPayload(req.Body, &servicesMeta, v, contentType); err != nil {
		log.Errorf("cannot decode %s services payload: %v", v, err)
		HTTPDecodingError(err, []string{tagServiceHandler, fmt.Sprintf("v:%s", v)}, w)
		return
	}

	HTTPOK(w)

	// We parse the tags from the header
	tags := info.Tags{
		Lang:          req.Header.Get("Datadog-Meta-Lang"),
		LangVersion:   req.Header.Get("Datadog-Meta-Lang-Version"),
		Interpreter:   req.Header.Get("Datadog-Meta-Lang-Interpreter"),
		TracerVersion: req.Header.Get("Datadog-Meta-Tracer-Version"),
	}

	// We get the address of the struct holding the stats associated to the tags
	ts := r.stats.GetTagStats(tags)

	atomic.AddInt64(&ts.ServicesReceived, int64(len(servicesMeta)))

	bytesRead := req.Body.(*model.LimitedReader).Count
	if bytesRead > 0 {
		atomic.AddInt64(&ts.ServicesBytes, int64(bytesRead))
	}

	r.services <- servicesMeta
}

// logStats periodically submits stats about the receiver to statsd
func (r *HTTPReceiver) logStats() {
	var lastLog time.Time
	accStats := info.NewReceiverStats()

	for now := range time.Tick(10 * time.Second) {
		statsd.Client.Gauge("datadog.trace_agent.heartbeat", 1, []string{"version:" + info.Version}, 1)

		// We update accStats with the new stats we collected
		accStats.Acc(r.stats)

		// Publish the stats accumulated during the last flush
		r.stats.Publish()

		// We reset the stats accumulated during the last 10s.
		r.stats.Reset()

		if now.Sub(lastLog) >= time.Minute {
			// We expose the stats accumulated to expvar
			info.UpdateReceiverStats(accStats)

			for _, logStr := range accStats.Strings() {
				log.Info(logStr)
			}

			// We reset the stats accumulated during the last minute
			accStats.Reset()
			lastLog = now

			// Also publish rates by service (they are updated by receiver)
			rates := r.dynConf.RateByService.GetAll()
			info.UpdateRateByService(rates)
		}
	}
}

// Languages returns the list of the languages used in the traces the agent receives.
func (r *HTTPReceiver) Languages() string {
	// We need to use this map because we can have several tags for a same language.
	langs := make(map[string]bool)
	str := []string{}

	r.stats.RLock()
	for tags := range r.stats.Stats {
		if _, ok := langs[tags.Lang]; !ok {
			str = append(str, tags.Lang)
			langs[tags.Lang] = true
		}
	}
	r.stats.RUnlock()

	sort.Strings(str)
	return strings.Join(str, "|")
}

func getTraces(v APIVersion, w http.ResponseWriter, req *http.Request) (model.Traces, bool) {
	var traces model.Traces
	contentType := req.Header.Get("Content-Type")

	switch v {
	case v01:
		// We cannot use decodeReceiverPayload because []model.Span does not
		// implement msgp.Decodable. This hack can be removed once we
		// drop v01 support.
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			log.Errorf("rejecting client request, unsupported media type %q", contentType)
			HTTPFormatError([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return nil, false
		}

		// in v01 we actually get spans that we have to transform in traces
		var spans []model.Span
		if err := json.NewDecoder(req.Body).Decode(&spans); err != nil {
			log.Errorf("cannot decode %s traces payload: %v", v, err)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return nil, false
		}
		traces = model.TracesFromSpans(spans)
	case v02:
		fallthrough
	case v03:
		fallthrough
	case v04:
		if err := decodeReceiverPayload(req.Body, &traces, v, contentType); err != nil {
			log.Errorf("cannot decode %s traces payload: %v", v, err)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return nil, false
		}
	default:
		HTTPEndpointNotSupported([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
		return nil, false
	}

	return traces, true
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

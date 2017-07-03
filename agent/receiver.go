package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/tinylib/msgp/msgp"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
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
)

// HTTPReceiver is a collector that uses HTTP protocol and just holds
// a chan where the spans received are sent one by one
type HTTPReceiver struct {
	traces   chan model.Trace
	services chan model.ServicesMetadata
	conf     *config.AgentConfig

	stats      *receiverStats
	preSampler *sampler.PreSampler

	exit chan struct{}

	maxRequestBodyLength int64
	debug                bool
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver(conf *config.AgentConfig) *HTTPReceiver {
	// use buffered channels so that handlers are not waiting on downstream processing
	return &HTTPReceiver{
		traces:     make(chan model.Trace, 5000), // about 1000 traces/sec for 5 sec
		services:   make(chan model.ServicesMetadata, 50),
		conf:       conf,
		stats:      newReceiverStats(),
		preSampler: sampler.NewPreSampler(conf.PreSampleRate),
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

	go func() {
		r.preSampler.Run()
	}()

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

	go func() {
		defer watchdog.LogOnPanic()
		stoppableListener.Refresh(r.conf.ConnectionLimit)
	}()
	go func() {
		defer watchdog.LogOnPanic()
		server.Serve(stoppableListener)
	}()

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
			log.Errorf("rejecting client request, unsupported media type %q", contentType)
			HTTPFormatError([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}

		f(v, w, req)
	})
}

// handleTraces knows how to handle a bunch of traces
func (r *HTTPReceiver) handleTraces(v APIVersion, w http.ResponseWriter, req *http.Request) {
	if !r.preSampler.Sample(req) {
		HTTPOK(w)
		return
	}

	var traces model.Traces
	contentType := req.Header.Get("Content-Type")
	headerFields := map[string]string{
		"lang":           req.Header.Get("Datadog-Meta-Lang"),
		"lang_version":   req.Header.Get("Datadog-Meta-Lang-Version"),
		"interpreter":    req.Header.Get("Datadog-Meta-Lang-Interpreter"),
		"tracer_version": req.Header.Get("Datadog-Meta-Tracer-Version"),
	}
	hash, err := getHash(headerFields)
	if err != nil {
		log.Errorf("Couldn't parse the header tags into a hash: %v", err)
		return
	}

	switch v {
	case v01:
		// We cannot use decodeReceiverPayload because []model.Span does not
		// implement msgp.Decodable. This hack can be removed once we
		// drop v01 support.
		if contentType != "application/json" && contentType != "text/json" && contentType != "" {
			log.Errorf("rejecting client request, unsupported media type %q", contentType)
			HTTPFormatError([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}

		// in v01 we actually get spans that we have to transform in traces
		var spans []model.Span
		if err := json.NewDecoder(req.Body).Decode(&spans); err != nil {
			log.Errorf("cannot decode %s traces payload: %v", v, err)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}
		traces = model.TracesFromSpans(spans)

	case v02:
		fallthrough
	case v03:
		if err := decodeReceiverPayload(req.Body, &traces, v, contentType); err != nil {
			log.Errorf("cannot decode %s traces payload: %v", v, err)
			HTTPDecodingError(err, []string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
			return
		}

	default:
		HTTPEndpointNotSupported([]string{tagTraceHandler, fmt.Sprintf("v:%s", v)}, w)
		return
	}

	// We successfuly decoded the payload
	HTTPOK(w)

	// We create a new PayloadStats struct where we will store the stats for this payload
	ps := newPayloadStats()
	ps.tags = getTags(headerFields)

	bytesRead := req.Body.(*model.LimitedReader).Count
	if bytesRead > 0 {
		ps.stats["traces_bytes"] = int64(bytesRead)
	}

	// normalize data
	for i := range traces {
		spans := len(traces[i])
		normTrace, err := model.NormalizeTrace(traces[i])
		if err != nil {
			ps.stats["traces_dropped"] += 1
			ps.stats["spans_dropped"] += int64(spans)

			errorMsg := fmt.Sprintf("dropping trace reason: %s (debug for more info), %v", err, normTrace)
			if len(errorMsg) > 150 && r.debug {
				errorMsg = errorMsg[:150] + "..."
			}
			log.Errorf(errorMsg)
		} else {
			ps.stats["spans_dropped"] += int64(spans - len(normTrace))

			// if our downstream consumer is slow, we drop the trace on the floor
			// this is a safety net against us using too much memory
			// when clients flood us
			select {
			case r.traces <- normTrace:
			default:
				ps.stats["traces_dropped"] += 1
				ps.stats["spans_dropped"] += int64(spans)

				log.Errorf("dropping trace reason: rate-limited")
			}
		}

		ps.stats["traces_received"] += 1
		ps.stats["spans_received"] += int64(spans)
	}

	r.stats.updateStats(hash, ps)
}

// handleServices handle a request with a list of several services
func (r *HTTPReceiver) handleServices(v APIVersion, w http.ResponseWriter, req *http.Request) {

	var servicesMeta model.ServicesMetadata

	contentType := req.Header.Get("Content-Type")
	headerFields := map[string]string{
		"lang":           req.Header.Get("Datadog-Meta-Lang"),
		"lang_version":   req.Header.Get("Datadog-Meta-Lang-Version"),
		"interpreter":    req.Header.Get("Datadog-Meta-Lang-Interpreter"),
		"tracer_version": req.Header.Get("Datadog-Meta-Tracer-Version"),
	}
	hash, err := getHash(headerFields)
	if err != nil {
		log.Errorf("Couldn't parse the header tags into a hash: %v", err)
		return
	}
	if err := decodeReceiverPayload(req.Body, &servicesMeta, v, contentType); err != nil {
		log.Errorf("cannot decode %s services payload: %v", v, err)
		HTTPDecodingError(err, []string{tagServiceHandler, fmt.Sprintf("v:%s", v)}, w)
		return
	}

	statsd.Client.Count("datadog.trace_agent.receiver.service", int64(len(servicesMeta)), nil, 1)
	HTTPOK(w)

	// We create a new PayloadStats struct where we will store the stats for this payload
	ps := newPayloadStats()
	ps.tags = getTags(headerFields)

	bytesRead := req.Body.(*model.LimitedReader).Count
	if bytesRead > 0 {
		ps.stats["services_bytes"] = int64(bytesRead)
	}

	r.services <- servicesMeta
	r.stats.updateStats(hash, ps)
}

// logStats periodically submits stats about the receiver to statsd
func (r *HTTPReceiver) logStats() {
	//var accStats receiverStats
	//var lastLog time.Time

	for _ = range time.Tick(10 * time.Second) {
		statsd.Client.Gauge("datadog.trace_agent.heartbeat", 1, []string{"version:" + Version}, 1)

		// Publish the stats accumulated during the last flush
		r.stats.publish()

		//if now.Sub(lastLog) >= time.Minute {
		//	updateReceiverStats(accStats)
		//	log.Infof("receiver handled %d spans, dropped %d ; handled %d traces, dropped %d",
		//		accStats.SpansReceived, accStats.SpansDropped,
		//		accStats.TracesReceived, accStats.TracesDropped)

		//	accStats = receiverStats{}
		//	lastLog = now
		//}
	}
}

type receiverStats struct {
	sync.Mutex
	stats map[uint64]payloadStats
}

func newReceiverStats() *receiverStats {
	return &receiverStats{sync.Mutex{}, map[uint64]payloadStats{}}
}

func (s *receiverStats) updateStats(hash uint64, stats *payloadStats) {
	newStats, ok := s.getStats(hash)
	if !ok {
		s.setStats(hash, stats)
		return
	}

	for k, v := range stats.stats {
		newStats.stats[k] += v
	}
	s.setStats(hash, newStats)
}

func (s *receiverStats) getStats(hash uint64) (*payloadStats, bool) {
	s.Lock()
	defer s.Unlock()
	if stats, ok := s.stats[hash]; ok {
		return stats.safeCopy(), true
	} else {
		return nil, false
	}
}

func (s *receiverStats) setStats(hash uint64, stats *payloadStats) {
	s.Lock()
	defer s.Unlock()
	s.stats[hash] = *stats
}

func (s *receiverStats) publish() {
	s.Lock()
	defer s.Unlock()
	for _, payloadStats := range s.stats {
		payloadStats.publish()
	}

	// We reset the stats accumulated during the last 10s.
	s.stats = map[uint64]payloadStats{}
}

// payloadStats contains stats about the volume of data received for one specific payload
type payloadStats struct {
	stats map[string]int64
	tags  []string
}

func newPayloadStats() *payloadStats {
	stats := map[string]int64{
		// traces_bytes is the amount of data received on the traces endpoint (raw data, encoded, compressed).
		"traces_bytes": 0,
		// ServicesBytes is the amount of data received on the services endpoint (raw data, encoded, compressed).
		"services_bytes": 0,
		// SpansReceived is the number of spans received, including the dropped ones
		"spans_received": 0,
		// TracesReceived is the number of traces received, including the dropped ones
		"traces_received": 0,
		// SpansDropped is the number of spans dropped
		"spans_dropped": 0,
		// SpansReceived is the number of traces dropped
		"traces_dropped": 0,
	}
	return &payloadStats{stats, []string{}}
}

func (s *payloadStats) safeCopy() *payloadStats {
	copy := newPayloadStats()
	for k, v := range s.stats {
		copy.stats[k] = v
	}
	copy.tags = s.tags
	return copy
}

func (s *payloadStats) publish() {
	template := "datadog.trace_agent.receiver.%s"
	for k, v := range s.stats {
		var tags []string
		if k == "services" {
			tags = append(s.tags, "endpoint:services")
		} else {
			tags = append(s.tags, "endpoint:traces")
		}
		statsd.Client.Count(fmt.Sprintf(template, k), v, tags, 1)
	}
}

func getTags(headerFields map[string]string) []string {
	tags := []string{}
	for k, v := range headerFields {
		if v != "" {
			tags = append(tags, fmt.Sprintf("%s:%s", k, v))
		}
	}
	return tags
}

// getHash returns the hash of the tag map
func getHash(tags map[string]string) (uint64, error) {
	h := fnv.New64()
	bytes, err := getBytes(tags)
	if err != nil {
		return 0, err
	}
	h.Write(bytes)
	return h.Sum64(), nil
}

// getBytes return the binary version of any interface
func getBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

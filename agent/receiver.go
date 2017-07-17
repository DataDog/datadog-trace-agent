package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"strconv"
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

var (
	// headerFields is a map used to decode the header metas
	headerFields = map[string]string{
		"lang":           "Datadog-Meta-Lang",
		"lang_version":   "Datadog-Meta-Lang-Version",
		"interpreter":    "Datadog-Meta-Lang-Interpreter",
		"tracer_version": "Datadog-Meta-Tracer-Version",
	}

	// initStats is the map we use to initialize the payload stats
	initStats = map[string]int64{
		// traces_bytes is the amount of data received on the traces endpoint (raw data, encoded, compressed).
		"traces_bytes": 0,
		// services_bytes is the amount of data received on the services endpoint (raw data, encoded, compressed).
		"services_bytes": 0,
		// spans_received is the number of spans received, including the dropped ones
		"spans_received": 0,
		// traces_received is the number of traces received, including the dropped ones
		"traces_received": 0,
		// spans_dropped is the number of spans dropped
		"spans_dropped": 0,
		// traces_dropped is the number of traces dropped
		"traces_dropped": 0,
	}
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

	traces, ok := getTraces(v, w, req)
	if !ok {
		return
	}

	HTTPOK(w) // We successfuly decoded the payload

	// We create a new PayloadStats struct where we will store the stats for this payload
	ps := newPayloadStats(req)

	bytesRead := req.Body.(*model.LimitedReader).Count
	if bytesRead > 0 {
		ps.stats["traces_bytes"] = int64(bytesRead)
	}

	// normalize data
	for i := range traces {
		spans := len(traces[i])

		ps.stats["traces_received"] += 1
		ps.stats["spans_received"] += int64(spans)

		normTrace, err := model.NormalizeTrace(traces[i])
		if err != nil {
			ps.stats["traces_dropped"] += 1
			ps.stats["spans_dropped"] += int64(spans)

			errorMsg := fmt.Sprintf("dropping trace reason: %s (debug for more info), %v", err, normTrace)

			// avoid truncation in DEBUG mode
			if len(errorMsg) > 150 && !r.debug {
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

	}

	r.stats.updateStats(ps)
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
	statsd.Client.Count("datadog.trace_agent.receiver.service", int64(len(servicesMeta)), nil, 1)
	HTTPOK(w)

	// We create a new PayloadStats struct where we will store the stats for this payload
	ps := newPayloadStats(req)

	bytesRead := req.Body.(*model.LimitedReader).Count
	if bytesRead > 0 {
		ps.stats["services_bytes"] = int64(bytesRead)
	}

	r.services <- servicesMeta

	// We update the stats
	r.stats.updateStats(ps)
}

// logStats periodically submits stats about the receiver to statsd
func (r *HTTPReceiver) logStats() {
	var lastLog time.Time
	accStats := newReceiverStats()

	for now := range time.Tick(10 * time.Second) {
		statsd.Client.Gauge("datadog.trace_agent.heartbeat", 1, []string{"version:" + Version}, 1)

		// We update accStats with the new stats we collected
		accStats.acc(r.stats)
		r.stats.Lock()
		log.Infof("ReceiverStats: %s", r.stats.String())
		r.stats.Unlock()
		log.Infof("AccStats: %s", accStats.String())

		// Publish the stats accumulated during the last flush and reset r.stats
		r.stats.publish()

		// We reset the stats accumulated during the last 10s.
		r.stats.reset()

		if now.Sub(lastLog) >= time.Minute {
			// Here we log the stats we accumulated
			log.Infof("Accumulated stats (for 1 minute): %s", accStats.String())

			// We reset the stats accumulated during the last minute
			accStats.reset()
			lastLog = now
		}
	}
}

type receiverStats struct {
	sync.Mutex
	stats map[uint64]payloadStats
}

func newReceiverStats() *receiverStats {
	return &receiverStats{sync.Mutex{}, map[uint64]payloadStats{}}
}

func (s *receiverStats) String() string {
	str := ""
	for k, v := range s.stats {
		str += strconv.FormatUint(k, 10) + v.String()
	}
	return str
}

func (accStats *receiverStats) acc(rs *receiverStats) {
	rs.Lock()
	defer rs.Unlock()
	for _, payloadStats := range rs.stats {
		accStats.updateStats(&payloadStats)
	}
}

func (s *receiverStats) updateStats(ps *payloadStats) {
	log.Infof("stats before update: %v", s.String())
	payloadStats, ok := s.getPayloadStats(ps.hash)
	if !ok {
		// No stats for this hash yet
		log.Infof("No stats for this hash: %v", ps.hash)
		s.setStats(ps)
		return
	}

	// We have to update the stats we already gathered for this hash
	for k, v := range ps.stats {
		payloadStats.stats[k] += v
	}
	s.setStats(payloadStats)
}

func (s *receiverStats) getPayloadStats(hash uint64) (*payloadStats, bool) {
	s.Lock()
	defer s.Unlock()
	if payloadStats, ok := s.stats[hash]; ok {
		return payloadStats.clone(), true
	} else {
		return nil, false
	}
}

func (s *receiverStats) clone() map[uint64]payloadStats {
	s.Lock()
	defer s.Unlock()
	stats := make(map[uint64]payloadStats, len(s.stats))
	for k, v := range s.stats {
		stats[k] = v
	}
	return stats
}

func (s *receiverStats) setStats(ps *payloadStats) {
	s.Lock()
	defer s.Unlock()
	s.stats[ps.hash] = *ps
}

func (s *receiverStats) publish() {
	s.Lock()
	defer s.Unlock()
	for _, payloadStats := range s.stats {
		payloadStats.publish()
	}
}

func (s *receiverStats) reset() {
	s.Lock()
	defer s.Unlock()
	s.stats = map[uint64]payloadStats{}
}

// payloadStats contains stats specific to a tag set
type payloadStats struct {
	stats map[string]int64
	tags  []string
	hash  uint64
}

func newPayloadStats(req *http.Request) *payloadStats {
	stats := getInitStatsCopy()
	tags := getTags(req)
	hash := hash(tags)
	return &payloadStats{stats, tags, hash}
}

func (s *payloadStats) String() string {
	return fmt.Sprintf("%v: %v\n", s.tags, s.stats)
}

func (s *payloadStats) clone() *payloadStats {
	var stats map[string]int64
	for k, v := range s.stats {
		stats[k] = v
	}
	return &payloadStats{stats, s.tags, s.hash}
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

func getTags(req *http.Request) []string {
	tags := []string{}
	for meta, headerField := range headerFields {
		value := req.Header.Get(headerField)
		if value != "" {
			tags = append(tags, fmt.Sprintf("%s:%s", meta, value))
		}
	}
	return tags
}

// hash returns the hash of the tag slice
func hash(tags []string) uint64 {
	h := fnv.New64()
	s := strings.Join(tags, "")
	h.Write([]byte(s))
	return h.Sum64()
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

func getInitStatsCopy() map[string]int64 {
	stats := make(map[string]int64, len(initStats))
	for k, v := range initStats {
		stats[k] = v
	}
	return stats
}

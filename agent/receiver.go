package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/statsd"
	log "github.com/cihub/seelog"
)

// APIVersion is a dumb way to version our collector handlers
type APIVersion int

const (
	v01 APIVersion = iota
	v02
)

// receiverStats tracks statistics about incoming payloads
type receiverStats struct {
	Errors         int64
	SpansReceived  int64
	TracesReceived int64
	SpansDropped   int64
	TracesDropped  int64
}

// HTTPReceiver is a collector that uses HTTP protocol and just holds
// a chan where the spans received are sent one by one
type HTTPReceiver struct {
	traces   chan model.Trace
	services chan model.ServicesMetadata

	// internal telemetry
	stats receiverStats

	exit chan struct{}
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver() *HTTPReceiver {
	// use buffered channels so that handlers are not waiting on downstream processing
	return &HTTPReceiver{
		traces:   make(chan model.Trace, 50),
		services: make(chan model.ServicesMetadata, 50),
		exit:     make(chan struct{}),
	}
}

// Start actually starts the HTTP server and returns any error that could
// have arosen
func (l *HTTPReceiver) Run() {
	httpHandleWithVersion := func(v APIVersion, f func(APIVersion, http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			f(v, w, r)
		}
	}

	// legacy collector API
	http.HandleFunc("/spans", httpHandleWithVersion(v01, l.handleTraces))
	http.HandleFunc("/services", httpHandleWithVersion(v01, l.handleServices))

	// v0.1 collector API
	http.HandleFunc("/v0.1/spans", httpHandleWithVersion(v01, l.handleTraces))
	http.HandleFunc("/v0.1/services", httpHandleWithVersion(v01, l.handleServices))

	// v0.2 collector API
	http.HandleFunc("/v0.2/traces", httpHandleWithVersion(v02, l.handleTraces))
	http.HandleFunc("/v0.2/services", httpHandleWithVersion(v02, l.handleServices))

	addr := ":7777"
	log.Infof("listening for traces at http://localhost%s/", addr)

	tcpL, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("could not create TCP listener")
		panic(err)
	}

	sl, err := NewStoppableListener(tcpL, l.exit)
	// some clients might use keep-alive and keep open their connections too long
	// avoid leaks
	server := http.Server{ReadTimeout: 5 * time.Second}


	go l.logStats()
	server.Serve(sl)
}

// HTTPErrorAndLog outputs an HTTP error with a code, a description text + DD metric
func HTTPErrorAndLog(w http.ResponseWriter, code int, errClient string, err error, tags []string) {
	log.Errorf("request error, code:%d tags:%v err: %s", code, tags, err)
	tags = append(tags, fmt.Sprintf("code:%d", code))
	tags = append(tags, fmt.Sprintf("err:%s", errClient))
	statsd.Client.Count("trace_agent.receiver.error", 1, tags, 1)

	http.Error(w, errClient, code)
}

// HTTPOK just returns a OK in the response
func HTTPOK(w http.ResponseWriter, tags []string) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

// handleTraces knows how to handle a bunch of traces
func (l *HTTPReceiver) handleTraces(v APIVersion, w http.ResponseWriter, r *http.Request) {
	var traces []model.Trace
	dec := json.NewDecoder(r.Body)

	mTags := []string{"handler:traces", fmt.Sprintf("v:%d", v)}

	switch v {
	case v01:
		// in v01 we actually get spans that we have to transform in traces
		var spans []model.Span
		err := dec.Decode(&spans)
		if err != nil {
			HTTPErrorAndLog(w, 500, "decoding-error", err, mTags)
			return
		}

		byID := make(map[uint64][]model.Span)
		for _, s := range spans {
			byID[s.TraceID] = append(byID[s.TraceID], s)
		}
		for _, t := range byID {
			traces = append(traces, t)
		}

	case v02:
		err := dec.Decode(&traces)
		if err != nil {
			HTTPErrorAndLog(w, 500, "decoding-error", err, mTags)
			return
		}
	}

	HTTPOK(w, mTags)

	// ensure all spans are OK
	// drop invalid spans

	var stotal, sdropped, ttotal, tdropped int
	for _, t := range traces {
		var toRemove []int
		var id uint64
		for i, s := range t {
			// we should drop "traces" that are not actually traces where several
			// trace IDs are reported. (probably a bug in the client)
			if i != 0 && s.TraceID != id {
				toRemove = make([]int, len(t)) // FIXME[leo]: non-needed alloc
			}
			id = s.TraceID

			err := s.Normalize()
			if err != nil {
				log.Errorf("dropping span %v, could not normalize: %v", s, err)
				toRemove = append(toRemove, i)
			}
		}

		stotal += len(t)
		sdropped += len(toRemove)

		// empty traces or we remove everything
		if len(toRemove) == len(t) {
			tdropped++
			continue
		}

		for _, idx := range toRemove {
			t[idx] = t[len(t)-1]
			t = t[:len(t)-1]
		}

		log.Debugf("received a trace, id:%d spans:%d", t[0].TraceID, len(t))
		l.traces <- t
		ttotal++
	}

	// Log stats
	atomic.AddInt64(&l.stats.TracesReceived, int64(ttotal))
	atomic.AddInt64(&l.stats.SpansReceived, int64(stotal))
	atomic.AddInt64(&l.stats.TracesDropped, int64(tdropped))
	atomic.AddInt64(&l.stats.SpansDropped, int64(sdropped))
}

// handleServices handle a request with a list of several services
func (l *HTTPReceiver) handleServices(v APIVersion, w http.ResponseWriter, r *http.Request) {
	mTags := []string{"handler:services"}

	dec := json.NewDecoder(r.Body)
	var servicesMeta model.ServicesMetadata

	err := dec.Decode(&servicesMeta)
	if err != nil {
		HTTPErrorAndLog(w, 500, "decoding-error", err, mTags)
		return
	}

	statsd.Client.Count("trace_agent.receiver.service", int64(len(servicesMeta)), nil, 1)
	HTTPOK(w, mTags)

	l.services <- servicesMeta
}

// logStats periodically submits stats about the receiver to statsd
func (l *HTTPReceiver) logStats() {
	for range time.Tick(10 * time.Second) {
		// Load counters and reset them for the next flush
		spans := atomic.LoadInt64(&l.stats.SpansReceived)
		l.stats.SpansReceived = 0

		traces := atomic.LoadInt64(&l.stats.TracesReceived)
		l.stats.TracesReceived = 0

		sdropped := atomic.LoadInt64(&l.stats.SpansDropped)
		l.stats.SpansDropped = 0

		tdropped := atomic.LoadInt64(&l.stats.TracesDropped)
		l.stats.TracesDropped = 0

		statsd.Client.Count("trace_agent.receiver.span", spans, nil, 1)
		statsd.Client.Count("trace_agent.receiver.trace", traces, nil, 1)
		statsd.Client.Count("trace_agent.receiver.span_dropped", sdropped, nil, 1)
		statsd.Client.Count("trace_agent.receiver.trace_dropped", tdropped, nil, 1)

		log.Infof("receiver handled %d spans, dropped %d ; handled %d traces, dropped %d", spans, sdropped, traces, tdropped)
	}
}

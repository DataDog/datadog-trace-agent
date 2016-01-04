package main

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/statsd"
	log "github.com/cihub/seelog"
)

// Receiver is the common interface for an agent span collector, it receives spans from clients
type Receiver interface {
	Start()
	Stop()
}

// HTTPReceiver is a collector that uses HTTP protocol and just holds
// a chan where the spans received are sent one by one
type HTTPReceiver struct {
	out chan model.Span

	Worker
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver() *HTTPReceiver {
	l := &HTTPReceiver{
		out: make(chan model.Span),
	}
	l.Init()
	return l
}

// Start actually starts the HTTP server and returns any error that could
// have arosen
func (l *HTTPReceiver) Start() {
	http.HandleFunc("/span", l.handleSpan)
	http.HandleFunc("/spans", l.handleSpans)
	addr := ":7777"
	log.Infof("HTTP Listener starting on %s", addr)

	tcpL, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("Could not create TCP listener")
		panic(err)
	}

	sl, err := NewStoppableListener(tcpL, l.exit)
	// some clients might use keep-alive and keep open their connections too long
	// avoid leaks
	server := http.Server{ReadTimeout: 5 * time.Second}

	l.wg.Add(1)
	defer l.wg.Done()

	go server.Serve(sl)
}

// handleSpan handle a request with a single span
func (l *HTTPReceiver) handleSpan(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var s model.Span
	err := decoder.Decode(&s)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))

	// FIXME[matt] this can return an error which is clearly ignored.
	s.Normalize()

	l.out <- s
}

// handleSpans handle a request with a list of several spans
func (l *HTTPReceiver) handleSpans(w http.ResponseWriter, r *http.Request) {
	statsd.Client.Count("trace_agent.receiver.payload", 1, nil, 1)

	decoder := json.NewDecoder(r.Body)
	var spans []model.Span
	err := decoder.Decode(&spans)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		statsd.Client.Count("trace_agent.receiver.error", 1, []string{"error:unmarshal-json"}, 1)
		return
	}

	statsd.Client.Count("trace_agent.receiver.span", int64(len(spans)), nil, 1)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))

	for _, s := range spans {
		err := s.Normalize()
		if err != nil {
			statsd.Client.Count("trace_agent.receiver.error", 1, []string{"error:normalize"}, 1)
			log.Errorf("Dropped a span, could not normalize span: %v", s)
			continue
		}

		log.Debugf("Received a span %v", s)
		l.out <- s
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"

	"github.com/DataDog/raclette/model"
	log "github.com/cihub/seelog"
)

// Receiver is the common interface for an agent span collector, it receives spans from clients
type Receiver interface {
	Start()
}

// HTTPReceiver is a collector that uses HTTP protocol and just holds
// a chan where the spans received are sent one by one
type HTTPReceiver struct {
	out       chan model.Span
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// NewHTTPReceiver returns a pointer to a new HTTPReceiver
func NewHTTPReceiver(exit chan struct{}, exitGroup *sync.WaitGroup) (*HTTPReceiver, chan model.Span) {
	r := HTTPReceiver{
		out:       make(chan model.Span),
		exit:      exit,
		exitGroup: exitGroup,
	}
	return &r, r.out
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

	sl, err := NewStoppableListener(tcpL)
	server := http.Server{}

	l.exitGroup.Add(1)
	defer l.exitGroup.Done()

	// Will return when closed using exit channels
	server.Serve(sl)
}

// handleSpan handle a request with a single span
func (l *HTTPReceiver) handleSpan(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(fmt.Errorf("Error writing header: %s", err))
	}

	var s model.Span
	//log.Printf("%s", body)
	err = json.Unmarshal(body, &s)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(fmt.Errorf("%s", err))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))

	s.Normalize()
	//	log.Infof("Span received. TraceID: %d, SpanID: %d, ParentID: %d, Start: %s, Service: %s, Type: %s",
	//		s.TraceID, s.SpanID, s.ParentID, s.FormatStart(), s.Service, s.Type)

	l.out <- s
}

// handleSpans handle a request with a list of several spans
func (l *HTTPReceiver) handleSpans(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(fmt.Errorf("%s", err))
	}

	var spans []model.Span
	err = json.Unmarshal(body, &spans)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(fmt.Errorf("%s", err))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))

	for _, s := range spans {
		err := s.Normalize()
		if err != nil {
			log.Errorf("Dropped a span, could not normalize span: %v", s)
			continue
		}

		log.Debugf("Received a span %v", s)
		l.out <- s
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/DataDog/raclette/model"
	log "github.com/cihub/seelog"
)

// Listener is the common interface for an agent span collector
type Listener interface {
	Init(chan model.Span)
	Start() error
}

// HTTPListener is a collector that uses HTTP protocol and just holds
// a chan where the spans received are sent one by one
type HTTPListener struct {
	out chan model.Span
}

// NewHTTPListener returns a poiter to a new HTTPListener
func NewHTTPListener() *HTTPListener {
	return &HTTPListener{}
}

// Init is needed before using the listener to set channels
func (l *HTTPListener) Init(out chan model.Span) {
	l.out = out
}

// Start actually starts the HTTP server and returns any error that could
// have arosen
func (l *HTTPListener) Start() error {
	http.HandleFunc("/span", l.handleSpan)
	http.HandleFunc("/spans", l.handleSpans)
	addr := ":7777"
	log.Infof("HTTP Listener starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleSpan handle a request with a single span
func (l *HTTPListener) handleSpan(w http.ResponseWriter, r *http.Request) {
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
	log.Infof("Span received. TraceID: %d, SpanID: %d, ParentID: %d, Start: %s, Service: %s, Type: %s",
		s.TraceID, s.SpanID, s.ParentID, s.FormatStart(), s.Service, s.Type)

	l.out <- s
}

// handleSpans handle a request with a list of several spans
func (l *HTTPListener) handleSpans(w http.ResponseWriter, r *http.Request) {
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

	log.Infof("Set of spans received")

	for _, s := range spans {
		s.Normalize()
		log.Infof("Span received. TraceID: %d, SpanID: %d, ParentID: %d, Start: %s, Service: %s, Type: %s",
			s.TraceID, s.SpanID, s.ParentID, s.FormatStart(), s.Service, s.Type)

		l.out <- s
	}
}

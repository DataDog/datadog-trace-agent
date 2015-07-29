package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/cihub/seelog"
)

type Listener interface {
	Init(chan Span)
	Start() error
}

type HttpListener struct {
	out chan Span
}

func NewHttpListener() *HttpListener {
	return &HttpListener{}
}

func (l *HttpListener) Init(out chan Span) {
	l.out = out
}

func (l *HttpListener) Start() error {
	http.HandleFunc("/span", l.HandleSpan)
	http.HandleFunc("/spans", l.HandleSpans)
	addr := ":7777"
	log.Infof("HTTP Listener starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

func (l *HttpListener) HandleSpan(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(fmt.Errorf("Error writing header: %s", err))
	}

	var s Span
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

func (l *HttpListener) HandleSpans(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(fmt.Errorf("%s", err))
	}

	var spans []Span
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

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

type Listener interface {
	Init(chan Span)
	Start()
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

func (l *HttpListener) Start() {
	http.HandleFunc("/", l.HandlePayload)
	log.Print("Listener ready to server")
	log.Fatal(http.ListenAndServe(":7777", nil))
}

func (l *HttpListener) HandlePayload(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal(err)
	}

	var s Span
	err = json.Unmarshal(body, &s)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal(err)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))

	s.Normalize()

	l.out <- s
}

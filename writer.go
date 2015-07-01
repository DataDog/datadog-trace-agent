package main

import (
	"log"

	"github.com/olivere/elastic"
)

type Writer interface {
	Init(chan Span)
	Start()
}

type StdoutWriter struct {
	in chan Span
}

func NewStdoutWriter() *StdoutWriter {
	return &StdoutWriter{}
}

func (w *StdoutWriter) Init(in chan Span) {
	w.in = in
}

func (w *StdoutWriter) Start() {
	go func() {
		for s := range w.in {
			log.Printf("TraceID: %d, SpanID: %d, ParentID: %d, Start: %s, Type: %s",
				s.TraceID, s.SpanID, s.ParentID, s.FormatStart(), s.Type)
		}
	}()

	log.Print("Writer started")
}

type EsWriter struct {
	in chan Span
	es *elastic.Client
}

func NewEsWriter() *EsWriter {
	return &EsWriter{}
}

func (w *EsWriter) Init(in chan Span) {
	w.in = in
	client, err := elastic.NewClient(
		elastic.SetURL("http://localhost:9200", "http://localhost:19200"),
		elastic.SetSniff(false), // personal-chef ES isn't in cluster mode
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create the index, just to be sure
	_, err = client.CreateIndex("raclette").Do()
	if err != nil {
		// log.Fatal(err)
	}

	w.es = client
}

func (w *EsWriter) Start() {
	go func() {
		for s := range w.in {
			w.es.Index().Index("raclette").Type("span").BodyJson(s).Do()
		}
	}()

	log.Print("Writer started")
}

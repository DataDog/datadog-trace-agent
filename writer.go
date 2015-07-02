package main

import (
	"log"

	"github.com/olivere/elastic"
)

type Writer interface {
	Init(chan Span)
	Start()
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

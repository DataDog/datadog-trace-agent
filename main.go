package main

import (
	"flag"
	"log"
)

type RacletteAgent struct {
	Listener Listener
	Writer   Writer
	Spans    chan Span
}

func main() {

	var writerName = flag.String("writer", "sqlite", "Where to write the spans. Available: sqlite, es")
	flag.Parse()

	var writer Writer
	if *writerName == "es" {
		writer = NewEsWriter()
	} else {
		writer = NewSqliteWriter()
	}

	listener := NewHttpListener()
	channel := make(chan Span)

	agent := RacletteAgent{
		Listener: listener,
		Writer:   writer,
		Spans:    channel,
	}

	log.Print("Init")
	agent.Init()
	log.Print("Start")
	agent.Start()
}

func (a *RacletteAgent) Init() {
	a.Writer.Init(a.Spans)
	a.Listener.Init(a.Spans)
}

func (a *RacletteAgent) Start() {
	a.Writer.Start()
	a.Listener.Start()
}

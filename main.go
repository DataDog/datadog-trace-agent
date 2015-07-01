package main

import (
	"log"
)

type RacletteAgent struct {
	Listener Listener
	Writer   Writer
	Spans    chan Span
}

func main() {

	listener := NewHttpListener()
	writer := NewStdoutWriter()
	// writer := NewEsWriter()
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

package main

import (
	"fmt"
	"math/rand"
	"time"

	log "github.com/cihub/seelog"
)

func main() {

	logger, err := log.LoggerFromConfigAsFile("./seelog.xml")
	if err != nil {
		panic(fmt.Sprintf("Error loading logging config: %v", err))
	}
	log.ReplaceLogger(logger)

	// Seed rand
	rand.Seed(time.Now().UTC().UnixNano())

	listener := NewHttpListener()
	spans := make(chan Span)

	writer := NewAPIWriter()
	agent := RacletteAgent{
		Listener: listener,
		Writers:  []Writer{writer},
		Spans:    spans,
	}

	log.Info("Inititializing")
	agent.Init()
	log.Info("Starting")
	err = agent.Start()
	if err != nil {
		panic(fmt.Errorf("Error starting agent: %s", err))
	}
}

type RacletteAgent struct {
	Listener     Listener
	Writers      []Writer
	WritersChans []chan Span
	Spans        chan Span
}

func (a *RacletteAgent) Init() {
	for _, writer := range a.Writers {
		out := make(chan Span)
		a.WritersChans = append(a.WritersChans, out)
		writer.Init(out)
	}
	a.Listener.Init(a.Spans)
}

func (a *RacletteAgent) Start() error {
	for _, writer := range a.Writers {
		writer.Start()
	}
	go func() {
		for s := range a.Spans {
			for _, c := range a.WritersChans {
				c <- s
			}
		}
	}()
	return a.Listener.Start()
}

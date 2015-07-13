package main

import (
	"flag"
	"log"
	"math/rand"
	"strings"
	"time"
)

type RacletteAgent struct {
	Listener     Listener
	Writers      []Writer
	WritersChans []chan Span
	Spans        chan Span
}

func main() {

	var writerNames = flag.String("writers", "sqlite", "comma-separated list of writers for spans. Available: sqlite, es")
	flag.Parse()

	// Seed rand
	rand.Seed(time.Now().UTC().UnixNano())

	var writers []Writer

	for _, writerName := range strings.Split(*writerNames, ",") {
		switch writerName {
		case "es":
			writers = append(writers, NewEsWriter())
		case "sqlite":
			writers = append(writers, NewSqliteWriter())
		default:
			log.Printf("Unknown writer %s, skipping ", writerName)
		}
	}

	if len(writers) == 0 {
		log.Fatal("You must specify at least one writer")
	}

	listener := NewHttpListener()
	channel := make(chan Span)

	agent := RacletteAgent{
		Listener: listener,
		Writers:  writers,
		Spans:    channel,
	}

	log.Print("Init")
	agent.Init()
	log.Print("Start")
	agent.Start()
}

func (a *RacletteAgent) Init() {
	for _, writer := range a.Writers {
		out := make(chan Span)
		a.WritersChans = append(a.WritersChans, out)
		writer.Init(out)
	}
	a.Listener.Init(a.Spans)
}

func (a *RacletteAgent) Start() {
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
	a.Listener.Start()
}

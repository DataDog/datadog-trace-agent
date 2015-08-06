package main

import "github.com/DataDog/raclette/model"

// RacletteAgent is our agent
type RacletteAgent struct {
	Listener     Listener
	Writers      []Writer
	WritersChans []chan model.Span
	Spans        chan model.Span
}

// Init needs to be called to initialize channels
func (a *RacletteAgent) Init() {
	for _, writer := range a.Writers {
		out := make(chan model.Span)
		a.WritersChans = append(a.WritersChans, out)
		writer.Init(out)
	}
	a.Listener.Init(a.Spans)
}

// Start launches writers and the listener to loop forever and
// returns listener errors
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

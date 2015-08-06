package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/DataDog/raclette/model"
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

	listener := NewHTTPListener()
	spans := make(chan model.Span)

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

package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/cihub/seelog"
)

// dumb handler that closes a channel to exit cleanly from routines
func handleSignal(exit chan bool) {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan)
	for signal := range sigChan {
		switch signal {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Info("Received interruption signal")
			close(exit)
		}
	}
}

func main() {
	// Initialize logging
	logger, err := log.LoggerFromConfigAsFile("./seelog.xml")
	if err != nil {
		panic(fmt.Sprintf("Error loading logging config: %v", err))
	}
	log.ReplaceLogger(logger)

	// Seed rand
	rand.Seed(time.Now().UTC().UnixNano())

	agent := NewAgent()

	// Handle stops properly
	defer agent.Join()
	go handleSignal(agent.exit)

	err = agent.Init()
	if err != nil {
		log.Error("Error when initializing agent")
		panic(err)
	}

	agent.Start()
}

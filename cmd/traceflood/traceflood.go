package main

import (
	"bytes"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/ugorji/go/codec"
)

const (
	duration           = time.Second
	defaultHTTPTimeout = time.Second
)

var mh codec.MsgpackHandle

func main() {
	// flags
	randSeed := flag.Int64("seed", 1, "set the `seed` using rand.Seed func")
	tracesNumber := flag.Int("traces", 1000, "set how many traces should be generated per flush")
	flag.Parse()

	// initialization
	rand.Seed(*randSeed)
	client := &http.Client{
		Timeout: defaultHTTPTimeout,
	}

	// infinite loop; it expects a SIGINT/SIGTERM to be stopped
	for {
		// generate the trace
		traces := []model.Trace{}
		for i := 0; i < *tracesNumber; i++ {
			traces = append(traces, fixtures.RandomTrace())
		}

		// flood the agent
		buffer := &bytes.Buffer{}
		encoder := codec.NewEncoder(buffer, &mh)
		err := encoder.Encode(traces)
		if err != nil {
			log.Fatal()
			return
		}

		// prepare the client and send the payload
		log.Println("Flooding...")
		req, _ := http.NewRequest("POST", "http://localhost:7777/v0.3/traces", buffer)
		req.Header.Set("Content-Type", "application/msgpack")
		client.Do(req)

		// wait before next execution
		time.Sleep(duration)
	}
}

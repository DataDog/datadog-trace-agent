package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DataDog/raclette/model"
)

func handleSignal(exit chan bool) {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan)
	for signal := range sigChan {
		switch signal {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Println("Terminating")
			close(exit)
		}
	}
}

func main() {
	var tpm = flag.Int("tpm", 1000, "nb of traces per minute generated")
	flag.Parse()
	// seed the generator
	rand.Seed(time.Now().Unix())

	s := newFakeSobotka()

	exit := make(chan bool)
	go handleSignal(exit)

	tps := *tpm / 60
	tickChan := time.NewTicker(time.Second).C
	timerChan := time.NewTimer(60 * time.Second).C

	genTraces := 0

	for {
		select {
		case <-timerChan:
			log.Printf("generated %d traces this minute", genTraces)
			return
		case <-tickChan:
			log.Printf("Generate %d Traces this second\n", tps)
			assigned := 0
			for assigned < tps {
				batchSize := rand.Intn(tps)
				if batchSize > tps-assigned || batchSize == 0 {
					continue
				}
				go func() {
					// avg depth level ~ 20
					var traces []model.Span
					for i := 0; i < batchSize; i++ {
						generateTrace(s, 0, 0, &traces, 0, 0)
						genTraces++
					}

					body, err := json.Marshal(traces)
					if err != nil {
						panic(err)
					}

					resp, err := http.Post("http://localhost:7777/spans", "application/json", bytes.NewBuffer(body))
					if err != nil {
						log.Println("error submitting: ", err)
					} else {
						log.Println(resp.Status)
					}
				}()
				assigned += batchSize
			}
		case <-exit:
			return
		}
	}
}

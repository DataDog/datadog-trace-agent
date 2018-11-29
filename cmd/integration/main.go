package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/DataDog/datadog-trace-agent/test"
)

func main() {
	var r test.Runner
	if err := r.Start(); err != nil {
		log.Fatal(err)
	}

	conf, err := ioutil.ReadFile("/opt/datadog-agent/etc/datadog.yaml")
	if err != nil {
		log.Fatal(err)
	}
	if err := r.RunAgent(conf); err != nil {
		log.Fatal(err)
	}

	traceList := agent.Traces{
		agent.Trace{test.NewSpan(nil)},
		agent.Trace{test.NewSpan(nil)},
		agent.Trace{test.NewSpan(nil)},
		agent.Trace{test.NewSpan(nil)},
	}
	for i := 0; i < 10; i++ {
		go func() {
			if err := r.Post(traceList); err != nil {
				log.Fatal(err)
			}
		}()
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		log.Fatal(r.Shutdown(2 * time.Second))
	}()

	for p := range r.Out() {
		switch v := p.(type) {
		case agent.TracePayload:
			fmt.Printf("OK traces (host:%q, count:%d)\n", v.HostName, len(v.Traces))
			if err := r.Shutdown(time.Second); err != nil {
				log.Fatal(err)
			}
		case agent.StatsPayload:
			fmt.Println("OK stats: ", len(v.Stats))
		}
	}
}

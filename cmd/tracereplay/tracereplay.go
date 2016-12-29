package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/ugorji/go/codec"
)

const (
	tracesDuration     = time.Second / 5
	servicesDuration   = time.Second / 20
	defaultHTTPTimeout = time.Second
	tracesEndPoint     = "http://localhost:7777/v0.3/traces"
	servicesEndPoint   = "http://localhost:7777/v0.3/services"
)

var mh codec.MsgpackHandle
var tracesNotFound sync.Once
var servicesNotFound sync.Once

var opts struct {
	loop     bool
	traces   string
	services string
}

func sendTraces(client *http.Client, traces string) error {
	tracesFile, tracesErr := os.Open(opts.traces)
	if tracesErr != nil {
		tracesNotFound.Do(func() {
			log.Printf("unable to open traces log file '%s': %v\n", opts.traces, tracesErr)
		})
	}
	defer tracesFile.Close()

	if tracesFile != nil {
		var traces []model.Trace
		scanner := bufio.NewScanner(tracesFile)
		l := 0
		sent := 0
		nbTraces := 0
		nbSpans := 0
		for scanner.Scan() {
			l++
			inBuf := bytes.NewReader(scanner.Bytes())
			dec := json.NewDecoder(inBuf)
			err := dec.Decode(&traces)
			if err != nil {
				log.Printf("bad traces input %s:%d\n", traces, l)
				continue
			}
			outBuf := &bytes.Buffer{}
			encoder := codec.NewEncoder(outBuf, &mh)
			err = encoder.Encode(traces)
			if err != nil {
				log.Fatalf("unable to encode %s:%d\n", traces, l)
				return err
			}

			req, _ := http.NewRequest("POST", tracesEndPoint, outBuf)
			req.Header.Set("Content-Type", "application/msgpack")
			_, err = client.Do(req)
			if err != nil {
				log.Printf("client error: %v\n", err)
				continue
			}
			sent++
			nbTraces += len(traces)
			for _, trace := range traces {
				nbSpans += len(trace)
			}

			time.Sleep(tracesDuration)
		}
		log.Printf("traces: sent %d/%d payloads (%d traces, %d spans)", sent, l, nbTraces, nbSpans)
	}

	return nil
}

func sendServices(client *http.Client, services string) error {
	servicesFile, servicesErr := os.Open(opts.services)
	if servicesErr != nil {
		servicesNotFound.Do(func() {
			log.Printf("unable to open services log file '%s': %v\n", opts.services, servicesErr)
		})
	}
	defer servicesFile.Close()

	if servicesFile != nil {
		var services model.ServicesMetadata
		scanner := bufio.NewScanner(servicesFile)
		l := 0
		sent := 0
		for scanner.Scan() {
			l++
			inBuf := bytes.NewReader(scanner.Bytes())
			dec := json.NewDecoder(inBuf)
			err := dec.Decode(&services)
			if err != nil {
				log.Printf("bad services input %s:%d\n", services, l)
				continue
			}
			outBuf := &bytes.Buffer{}
			encoder := codec.NewEncoder(outBuf, &mh)
			err = encoder.Encode(services)
			if err != nil {
				log.Fatalf("bad services input %s:%d\n", services, l)
				return err
			}

			req, _ := http.NewRequest("POST", servicesEndPoint, outBuf)
			req.Header.Set("Content-Type", "application/msgpack")
			_, err = client.Do(req)
			if err != nil {
				log.Printf("client error: %v\n", err)
				continue
			}
			sent++
			time.Sleep(servicesDuration)
		}
		log.Printf("services: sent %d/%d payloads", sent, l)
	}

	return nil
}

func main() {
	done := make(chan struct{}, 2)

	// flags
	flag.BoolVar(&opts.loop, "loop", false, "Loop and keeping re-sending the same data over and over")
	flag.StringVar(&opts.traces, "traces", "traces.json", "Traces log file containing one JSON entry per line")
	flag.StringVar(&opts.services, "services", "services.json", "Services log file containing one JSON entry per line")
	flag.Parse()

	// initialization
	client := &http.Client{
		Timeout: defaultHTTPTimeout,
	}

	go func() {
		// infinite loop if loop is set to true; it expects a SIGINT/SIGTERM to be stopped
		for {
			sendTraces(client, opts.traces)
			if !opts.loop {
				break
			}
		}
		done <- struct{}{}
	}()

	go func() {
		// infinite loop if loop is set to true; it expects a SIGINT/SIGTERM to be stopped
		for {
			sendServices(client, opts.services)
			if !opts.loop {
				break
			}
		}
		done <- struct{}{}
	}()

	// Wait for traces & services to finish
	<-done
	<-done
}

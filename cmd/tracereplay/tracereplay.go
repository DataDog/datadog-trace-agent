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
	tracesDuration     = time.Second / 25
	servicesDuration   = time.Second / 33
	defaultHTTPTimeout = time.Second
	tracesEndPoint     = "http://localhost:7777/v0.3/traces"
	servicesEndPoint   = "http://localhost:7777/v0.3/services"
	bufferSize         = int(1e7)
)

var mh codec.MsgpackHandle
var tracesNotFound sync.Once
var servicesNotFound sync.Once

var opts struct {
	loop     bool
	traces   string
	services string
}

func makeTracesBuffers(tracesPath string) (error, [][]byte) {
	var traces []model.Trace
	var buffers [][]byte
	buf := make([]byte, bufferSize)

	tracesFile, tracesErr := os.Open(tracesPath)
	if tracesErr != nil {
		tracesNotFound.Do(func() {
			log.Printf("unable to open traces log file '%s': %v\n", opts.traces, tracesErr)
		})
		return nil, nil
	}
	defer tracesFile.Close()

	scanner := bufio.NewScanner(tracesFile)
	scanner.Buffer(buf, cap(buf)) // traces line can be very big, need a dedicated buffer
	nbPayloads := 0
	nbTraces := 0
	nbSpans := 0
	nbBytes := 0
	for scanner.Scan() {
		nbPayloads++
		inBuf := bytes.NewReader(scanner.Bytes())
		dec := json.NewDecoder(inBuf)
		err := dec.Decode(&traces)
		if err != nil {
			log.Printf("bad traces input %s:%d\n", traces, nbPayloads)
			continue
		}
		nbTraces += len(traces)
		for _, trace := range traces {
			nbSpans += len(trace)
		}
		outBuf := &bytes.Buffer{}
		encoder := codec.NewEncoder(outBuf, &mh)
		err = encoder.Encode(traces)
		if err != nil {
			log.Fatalf("unable to encode %s:%d\n", traces, nbPayloads)
			return err, nil
		}
		nbBytes += outBuf.Len()
		buffers = append(buffers, outBuf.Bytes())
	}
	log.Printf("traces: %d payloads %d traces %d spans %d bytes", nbPayloads, nbTraces, nbSpans, nbBytes)
	return nil, buffers
}

func sendTraces(client *http.Client, buffers [][]byte) error {

	sent := 0
	for _, buffer := range buffers {
		req, _ := http.NewRequest("POST", tracesEndPoint, bytes.NewReader(buffer))
		req.Header.Set("Content-Type", "application/msgpack")
		_, err := client.Do(req)
		if err != nil {
			log.Printf("client error: %v\n", err)
			continue
		}
		sent++

		time.Sleep(tracesDuration)
	}
	log.Printf("traces: sent %d/%d payloads", sent, len(buffers))

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
		_, buffers := makeTracesBuffers(opts.traces)
		if buffers != nil {
			// infinite loop if loop is set to true; it expects a SIGINT/SIGTERM to be stopped
			for {
				sendTraces(client, buffers)
				if !opts.loop {
					break
				}
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

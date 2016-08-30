package main

import (
	"sync"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	log "github.com/cihub/seelog"
)

// Agent struct holds all the sub-routines structs and make the data flow between them
type Agent struct {
	Receiver       *HTTPReceiver
	SublayerTagger *SublayerTagger
	Quantizer      *Quantizer
	Concentrator   *Concentrator
	Sampler        *Sampler
	Writer         *Writer

	// config
	Config *config.AgentConfig

	// Used to synchronize on a clean exit
	exit chan struct{}
}

// NewAgent returns a new Agent object, ready to be started
func NewAgent(conf *config.AgentConfig) *Agent {
	exit := make(chan struct{})

	r := NewHTTPReceiver(conf)
	st := NewSublayerTagger(r.traces)
	q := NewQuantizer(st.out)

	traceChans := traceFanOut(q.out, 2)
	c := NewConcentrator(traceChans[0], conf)
	s := NewSampler(traceChans[1], conf)

	w := NewWriter(conf, r.services)

	return &Agent{
		Config:         conf,
		Receiver:       r,
		SublayerTagger: st,
		Quantizer:      q,
		Concentrator:   c,
		Sampler:        s,
		Writer:         w,
		exit:           exit,
	}
}

// Run starts routers routines and individual pieces then stop them when the exit order is received
func (a *Agent) Run() {
	// Start all workers, last component first
	go a.Writer.Run()
	go a.runFlusher()
	go a.Sampler.Run()
	go a.Concentrator.Run()
	go a.Quantizer.Run()
	go a.SublayerTagger.Run()
	go a.Receiver.Run()

	<-a.exit
	log.Info("exiting")
	a.Stop()
}

// runFlusher periodically send a flush marker, collect the results and send the payload to the Writer
func (a *Agent) runFlusher() {
	ticker := time.NewTicker(a.Config.BucketInterval)
	for {
		select {
		case <-ticker.C:
			log.Debug("tick - agent triggering flush")
			a.Quantizer.out <- model.NewTraceFlushMarker()

			// Collect and merge partial flushs
			var wg sync.WaitGroup
			p := model.AgentPayload{}
			wg.Add(2)
			go func() {
				defer wg.Done()
				p.Stats = <-a.Concentrator.out
			}()
			go func() {
				defer wg.Done()
				traces := <-a.Sampler.out
				if a.Config.APIFlushTraces {
					p.Traces = traces
				} else {
					// try to avoid allocs
					p.Spans = make([]model.Span, 0, len(traces)*10)
					for _, t := range traces {
						p.Spans = append(p.Spans, t...)
					}
				}
			}()
			wg.Wait()
			log.Debugf("tock - all routines flushed (%d stats, %d/%d spans/traces)", len(p.Stats), len(p.Spans), len(p.Traces))

			if !p.IsEmpty() {
				a.Writer.in <- p
			} else {
				log.Debug("flush produced an empty payload, skipping")
			}
		case <-a.exit:
			ticker.Stop()
			return
		}
	}
}

// Stop stops all components
func (a *Agent) Stop() {
	log.Info("stopping the trace-agent")

	close(a.Receiver.exit)
	// this will stop in chain all of the agent components
	close(a.SublayerTagger.in)
	// flush last payload if possible
	close(a.Writer.exit)
}

// traceFanOut redistributes incoming traces to multiple components by returning multiple channels
func traceFanOut(in chan model.Trace, n int) []chan model.Trace {
	outChans := make([]chan model.Trace, 0, n)
	for i := 0; i < n; i++ {
		outChans = append(outChans, make(chan model.Trace))
	}
	go func() {
		for t := range in {
			for _, outc := range outChans {
				outc <- t
			}
		}
	}()

	return outChans
}

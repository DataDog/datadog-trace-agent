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
	CutoffFilter   *CutoffFilter
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
	cf := NewCutoffFilter(st.out, conf)
	q := NewQuantizer(cf.out)

	cChan, sChan := chanTPipe(q.out)
	c := NewConcentrator(cChan, conf)
	s := NewSampler(sChan, conf)

	w := NewWriter(conf)
	w.inServices = r.services

	return &Agent{
		Config:         conf,
		Receiver:       r,
		SublayerTagger: st,
		CutoffFilter:   cf,
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
	go a.CutoffFilter.Run()
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
			p := model.AgentPayload{
				HostName: a.Config.HostName,
				Env:      a.Config.DefaultEnv,
			}
			wg.Add(2)
			go func() {
				defer wg.Done()
				p.Stats = <-a.Concentrator.out
			}()
			go func() {
				defer wg.Done()
				traces := <-a.Sampler.out
				p.Traces = traces
			}()
			wg.Wait()
			log.Debugf("tock - all routines flushed (%d stats, %d traces)", len(p.Stats), len(p.Traces))

			if !p.IsEmpty() {
				a.Writer.inPayloads <- p
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

// chanTPipe redistributes incoming traces to multiple components by returning multiple channels
func chanTPipe(fromQuantizer chan model.Trace) (chan model.Trace, chan model.Trace) {
	toConcentrator := make(chan model.Trace)
	toSampler := make(chan model.Trace)

	go func() {
		for t := range fromQuantizer {
			t2 := make(model.Trace, len(t))
			copy(t2, t)

			for i := range t2 {
				if t2[i].Metrics == nil {
					continue
				}

				// this hack is needed because Metrics are read by the concentrator
				// (data from the sublayer tagger) and by the sampler which also writes
				// data to it. This avoids concurrent read/write map clashes.
				t2[i].Metrics = make(map[string]float64)
				for k, v := range t[i].Metrics {
					s2 := t2[i]
					s2.Metrics[k] = v
					t2[i] = s2
				}
			}

			toConcentrator <- t
			toSampler <- t2
		}
	}()

	return toConcentrator, toSampler
}

package main

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/quantizer"
)

// Quantizer generates meaningul resource for spans
type Quantizer struct {
	in  chan model.Trace
	out chan model.Trace

	Worker
}

// NewQuantizer creates a new Quantizer ready to be started
func NewQuantizer(in chan model.Trace) *Quantizer {
	q := &Quantizer{
		in:  in,
		out: make(chan model.Trace),
	}
	q.Init()
	return q
}

// Start runs the Quantizer by quantizing spans from the channel
func (q *Quantizer) Start() {
	q.wg.Add(1)
	go func() {
		for {
			select {
			case trace := <-q.in:
				for i, s := range trace {
					trace[i] = quantizer.Quantize(s)
				}
				q.out <- trace
			case <-q.exit:
				log.Info("Quantizer exiting")
				close(q.out)
				q.wg.Done()
				return
			}
		}
	}()

	log.Info("Quantizer started")
}

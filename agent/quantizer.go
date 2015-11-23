package main

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/quantizer"
)

// Quantizer generates meaningul resource for spans
type Quantizer struct {
	in  chan model.Span
	out chan model.Span

	Worker
}

// NewQuantizer creates a new Quantizer
func NewQuantizer(inSpans chan model.Span) *Quantizer {
	q := &Quantizer{
		in:  inSpans,
		out: make(chan model.Span),
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
			case span := <-q.in:
				q.out <- quantizer.Quantize(span)
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

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
	go func() {
		for span := range q.in {
			q.out <- quantizer.Quantize(span)
		}
	}()

	q.wg.Add(1)
	go func() {
		<-q.exit
		log.Info("Quantizer exiting")
		close(q.in)
		q.wg.Done()
		return
	}()

	log.Info("Quantizer started")
}

package main

import (
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/quantizer"
)

// Quantizer generates meaningul resource for spans
type Quantizer struct {
	in  chan model.Trace
	out chan model.Trace
}

// NewQuantizer creates a new Quantizer ready to be started
func NewQuantizer(in chan model.Trace) *Quantizer {
	return &Quantizer{
		in:  in,
		out: make(chan model.Trace),
	}
}

// Start runs the Quantizer by quantizing spans from the channel
func (q *Quantizer) Run() {
	for trace := range q.in {
		for i, s := range trace {
			trace[i] = quantizer.Quantize(s)
		}
		q.out <- trace
	}

	close(q.out)
}

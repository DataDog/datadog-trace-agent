package main

import (
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/quantizer"
)

// Quantizer generates meaningul resource for spans
type Quantizer struct {
	in        chan model.Span
	out       chan model.Span
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// NewQuantizer creates a new Quantizer
func NewQuantizer(inSpans chan model.Span, exit chan struct{}, exitGroup *sync.WaitGroup) *Quantizer {
	return &Quantizer{
		in:        inSpans,
		out:       make(chan model.Span),
		exit:      exit,
		exitGroup: exitGroup,
	}
}

// Start runs the Quantizer by quantizing spans from the channel
func (q *Quantizer) Start() {
	go func() {
		for span := range q.in {
			q.out <- quantizer.Quantize(span)
		}
	}()

	q.exitGroup.Add(1)
	go func() {
		<-q.exit
		log.Info("Quantizer exiting")
		close(q.in)
		q.exitGroup.Done()
		return
	}()

	log.Info("Quantizer started")
}

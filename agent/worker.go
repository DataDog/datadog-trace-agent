package main

import (
	// stdlib
	"sync"
)

// Worker is a generic structure inherited from all Agent components.
// It handles the proper initialization and exit of the worker.
type Worker struct {
	exit chan struct{}
	wg   *sync.WaitGroup
}

// Init initialize the worker synchornization.
func (w *Worker) Init() {
	w.exit = make(chan struct{})
	wg := sync.WaitGroup{}
	w.wg = &wg
}

// Stop is a blocking-stop of the worker.
func (w *Worker) Stop() {
	close(w.exit)
	w.wg.Wait()
}

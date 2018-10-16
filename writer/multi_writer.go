package writer

import (
	"sync"

	"github.com/DataDog/datadog-trace-agent/writer/config"
)

var _ PayloadSender = (*multiSender)(nil)

// multiSender is an implementation of PayloadSender which forwards any
// received payload to multiple PayloadSender.
type multiSender struct {
	senders []PayloadSender
	mwg     sync.WaitGroup // monitor consumer waitgroup
	mch     chan interface{}
}

// newMultiSender creates a new multiSender which will forward received
// payloads to all the given senders. When it comes to monitoring, the
// first sender is considered the main one.
func newMultiSender(senders []PayloadSender) *multiSender {
	return &multiSender{
		senders: senders,
		mch:     make(chan interface{}, len(senders)),
	}
}

// Start starts all senders.
func (w *multiSender) Start() {
	for _, sender := range w.senders {
		sender.Start()
	}
	for _, sender := range w.senders {
		w.mwg.Add(1)
		go func(ch <-chan interface{}) {
			defer w.mwg.Done()
			for event := range ch {
				w.mch <- event
			}
		}(sender.Monitor())
	}
}

// Stop stops all senders.
func (w *multiSender) Stop() {
	for _, sender := range w.senders {
		sender.Stop()
	}
	w.mwg.Wait()
	close(w.mch)
}

// Send forwards the payload to all registered senders.
func (w *multiSender) Send(p *Payload) {
	for _, sender := range w.senders {
		sender.Send(p)
	}
}

func (w *multiSender) Monitor() <-chan interface{} { return w.mch }

// Run implements PayloadSender.
func (w *multiSender) Run() { /* no-op */ }

func (w *multiSender) setEndpoint(endpoint Endpoint) {
	for _, sender := range w.senders {
		sender.setEndpoint(endpoint)
	}
}

func newSenderFactory(cfg config.QueuablePayloadSenderConf) func([]Endpoint) PayloadSender {
	return func(endpoints []Endpoint) PayloadSender {
		var senders []PayloadSender
		for _, e := range endpoints {
			senders = append(senders, NewCustomQueuablePayloadSender(e, cfg))
		}
		return newMultiSender(senders)
	}
}

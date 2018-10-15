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
}

// newMultiSender creates a new multiSender which will forward received
// payloads to all the given senders. When it comes to monitoring, the
// first sender is considered the main one.
func newMultiSender(senders []PayloadSender) *multiSender {
	return &multiSender{senders: senders}
}

// Start starts all senders.
func (w *multiSender) Start() {
	for _, sender := range w.senders {
		sender.Start()
	}
	// TODO(gbbr): improve Monitor() intent. Sender users are currently expected to consume the
	// channel. This shouldn't be an expectation as it will cause unexpected deadlocks.
	if len(w.senders) <= 1 {
		// The first sender monitor is already consumed via the Monitor() call.
		return
	}
	for i := 1; i < len(w.senders); i++ {
		w.mwg.Add(1)
		go func(ch <-chan interface{}) {
			defer w.mwg.Done()
			for range ch {
				// dismiss
			}
		}(w.senders[i].Monitor())
	}
}

// Stop stops all senders.
func (w *multiSender) Stop() {
	for _, sender := range w.senders {
		sender.Stop()
	}
	w.mwg.Wait()
}

// Send forwards the payload to all registered senders.
func (w *multiSender) Send(p *Payload) {
	for _, sender := range w.senders {
		s.Send(p)
	}
}

// Monitor returns the monitor for the first sender, which is considered
// to be targeting the main endpoint.
func (w *multiSender) Monitor() <-chan interface{} {
	if len(w.senders) == 0 {
		ch := make(chan interface{})
		close(ch)
		return ch
	}
	return w.senders[0].Monitor()
}

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

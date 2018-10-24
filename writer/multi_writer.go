package writer

import (
	"sync"

	"github.com/DataDog/datadog-trace-agent/writer/config"
)

var _ PayloadSender = (*multiSender)(nil)

// multiSender is an implementation of PayloadSender which forwards any
// received payload to multiple PayloadSenders, funnelling incoming monitor
// events.
type multiSender struct {
	senders []PayloadSender
	mwg     sync.WaitGroup    // monitor funnel waitgroup
	mch     chan monitorEvent // monitor funneling channel
}

// newMultiSender returns a new PayloadSender which forwards all sent payloads to all
// the given endpoints, as well as funnels all monitoring channels.
func newMultiSender(endpoints []Endpoint, cfg config.QueuablePayloadSenderConf) PayloadSender {
	if len(endpoints) == 1 {
		return newSender(endpoints[0], cfg)
	}
	senders := make([]PayloadSender, len(endpoints))
	for i, e := range endpoints {
		senders[i] = newSender(e, cfg)
	}
	return &multiSender{
		senders: senders,
		mch:     make(chan monitorEvent, len(senders)),
	}
}

// Start starts all senders.
func (w *multiSender) Start() {
	for _, sender := range w.senders {
		sender.Start()
	}
	for _, sender := range w.senders {
		w.mwg.Add(1)
		go func(ch <-chan monitorEvent) {
			defer w.mwg.Done()
			for event := range ch {
				w.mch <- event
			}
		}(sender.monitor())
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

func (w *multiSender) monitor() <-chan monitorEvent { return w.mch }

// Run implements PayloadSender.
func (w *multiSender) Run() { /* no-op */ }

func (w *multiSender) setEndpoint(endpoint Endpoint) {
	for _, sender := range w.senders {
		sender.setEndpoint(endpoint)
	}
}

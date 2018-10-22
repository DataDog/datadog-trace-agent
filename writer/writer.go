package writer

import "github.com/DataDog/datadog-trace-agent/config"

// BaseWriter encodes the base components and behaviour of a typical Writer.
type BaseWriter struct {
	payloadSender PayloadSender
	exit          chan struct{}
}

// NewBaseWriter creates a new instance of a BaseWriter.
func NewBaseWriter(conf *config.AgentConfig, path string, senderFactory func([]Endpoint) PayloadSender) *BaseWriter {
	endpoints := NewEndpoints(conf, path)
	return &BaseWriter{
		payloadSender: senderFactory(endpoints),
		exit:          make(chan struct{}),
	}
}

// Start starts the necessary components of a BaseWriter.
func (w *BaseWriter) Start() {
	w.payloadSender.Start()
}

// Stop stops any the stoppable components of a BaseWriter.
func (w *BaseWriter) Stop() {
	w.payloadSender.Stop()
}

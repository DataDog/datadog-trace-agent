package writer

import (
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
)

// BaseWriter encodes the base components and behaviour of a typical Writer.
type BaseWriter struct {
	payloadSender PayloadSender

	exit   chan struct{}
	exitWG *sync.WaitGroup

	conf *config.AgentConfig
}

// NewBaseWriter creates a new instance of a BaseWriter.
func NewBaseWriter(conf *config.AgentConfig, path string) *BaseWriter {
	return NewCustomSenderBaseWriter(conf, path, func(endpoint Endpoint) PayloadSender {
		return NewQueuablePayloadSender(endpoint)
	})
}

// NewCustomSenderBaseWriter creates a new instance of a BaseWriter with a custom sender.
func NewCustomSenderBaseWriter(conf *config.AgentConfig, path string,
	senderFactory func(Endpoint) PayloadSender) *BaseWriter {

	var endpoint Endpoint

	if conf.APIEnabled {
		client := NewClient(conf)
		endpoint = NewDatadogEndpoint(client, conf.APIEndpoint, path, conf.APIKey)
	} else {
		log.Info("API interface is disabled, flushing to /dev/null instead")
		endpoint = &NullEndpoint{}
	}

	return &BaseWriter{
		payloadSender: senderFactory(endpoint),
		exit:          make(chan struct{}),
		exitWG:        &sync.WaitGroup{},
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

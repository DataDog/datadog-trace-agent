package writer

import (
	"sync"

	"stackstate-trace-agent/statsd"
	log "github.com/cihub/seelog"

	"stackstate-trace-agent/config"
)

// BaseWriter encodes the base components and behaviour of a typical Writer.
type BaseWriter struct {
	payloadSender PayloadSender

	statsClient statsd.StatsClient

	exit   chan struct{}
	exitWG *sync.WaitGroup
}

// NewBaseWriter creates a new instance of a BaseWriter.
func NewBaseWriter(conf *config.AgentConfig, path string, senderFactory func(Endpoint) PayloadSender) *BaseWriter {
	var endpoint Endpoint

	if conf.APIEnabled {
		client := NewClient(conf)
		endpoint = NewStackStateEndpoint(client, conf.APIEndpoint, path, conf.APIKey)
	} else {
		log.Info("API interface is disabled, flushing to /dev/null instead")
		endpoint = &NullEndpoint{}
	}

	return &BaseWriter{
		payloadSender: senderFactory(endpoint),
		statsClient:   statsd.Client,
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

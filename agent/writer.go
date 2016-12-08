package main

import (
	"sync/atomic"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
)

// Writer is the last chain of trace-agent which takes the
// pre-processed data from channels and tentatively output them
// to a given endpoint.
type Writer struct {
	endpoint AgentEndpoint // where the data will end

	// input data
	inPayloads chan model.AgentPayload     // main payloads for processed traces/stats
	inServices chan model.ServicesMetadata // secondary services metadata

	payloadBuffer []model.AgentPayload   // buffered of payloads ready to send
	serviceBuffer model.ServicesMetadata // services are merged into this map continuously

	payloadBufferLen int32 // used for async debugging, not always exact

	exit chan struct{}
}

// NewWriter returns a new Writer
func NewWriter(conf *config.AgentConfig) *Writer {
	var endpoint AgentEndpoint

	if conf.APIEnabled {
		endpoint = NewAPIEndpoint(conf.APIEndpoints, conf.APIKeys)
	} else {
		log.Info("API interface is disabled, flushing to /dev/null instead")
		endpoint = NullEndpoint{}
	}

	return &Writer{
		endpoint: endpoint,

		// small buffer to not block in case we're flushing
		inPayloads: make(chan model.AgentPayload, 1),

		payloadBuffer: make([]model.AgentPayload, 0, 5),
		serviceBuffer: make(model.ServicesMetadata),

		exit: make(chan struct{}),
	}
}

// Run starts the writer and starts writing what comes through the
// input channel.
// NOTE: this currently happens sequentially, but it would not be too
// hard to mutex and parallelize. Not sure it's needed though.
func (w *Writer) Run() {
	for {
		select {
		case p := <-w.inPayloads:
			if p.IsEmpty() {
				continue
			}
			w.payloadBuffer = append(w.payloadBuffer, p)
			w.Flush()
		case sm := <-w.inServices:
			updated := w.serviceBuffer.Update(sm)
			if updated {
				w.FlushServices()
			}
		case <-w.exit:
			log.Info("exiting, trying to flush all remaining data")
			w.Flush()
			return
		}
		atomic.StoreInt32(&w.payloadBufferLen, int32(len(w.payloadBuffer)))
	}
}

// FlushServices initiate a flush of the services to the services endpoint
func (w *Writer) FlushServices() {
	w.endpoint.WriteServices(w.serviceBuffer)
}

// Flush actually writes the data in the API
func (w *Writer) Flush() {
	// TODO[leo]: batch payloads in same API key
	for _, p := range w.payloadBuffer {
		w.endpoint.Write(p)
	}
	// regardless if the http post was was success or not. We don't want to buffer
	// data in case of api outage. See also endpoint.Write() comment.
	w.payloadBuffer = nil
}

func (w *Writer) getPayloadBufferLen() int32 {
	return atomic.LoadInt32(&w.payloadBufferLen)
}

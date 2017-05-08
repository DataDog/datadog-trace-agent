package main

import (
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// the amount of time in seconds to wait before resending a payload
const payloadResendDelay = 5 * time.Second

// the amount of time in seconds a payload can stay buffered before being dropped
const payloadMaxAge = 10 * time.Minute

// writerPayload wraps a model.AgentPayload and keeps track of a list of
// endpoints the payload must be sent to.
type writerPayload struct {
	payload      model.AgentPayload // the payload itself
	size         int                // the size of the serialized payload or 0 if it has not been serialized yet
	endpoint     AgentEndpoint      // the endpoints the payload must be sent to
	creationDate time.Time          // the creation date of the payload
	nextFlush    time.Time          // The earliest moment we can flush
}

func newWriterPayload(p model.AgentPayload, endpoint AgentEndpoint) *writerPayload {
	return &writerPayload{
		payload:      p,
		endpoint:     endpoint,
		creationDate: time.Now(),
	}
}

func (p *writerPayload) write() error {
	size, err := p.endpoint.Write(p.payload)
	p.size = size
	return err
}

// Writer is the last chain of trace-agent which takes the
// pre-processed data from channels and tentatively output them
// to a given endpoint.
type Writer struct {
	endpoint AgentEndpoint // where the data will end

	// input data
	inPayloads chan model.AgentPayload     // main payloads for processed traces/stats
	inServices chan model.ServicesMetadata // secondary services metadata

	payloadBuffer []*writerPayload       // buffer of payloads ready to send
	serviceBuffer model.ServicesMetadata // services are merged into this map continuously

	exit   chan struct{}
	exitWG *sync.WaitGroup

	conf *config.AgentConfig
}

// NewWriter returns a new Writer
func NewWriter(conf *config.AgentConfig) *Writer {
	var endpoint AgentEndpoint

	if conf.APIEnabled {
		endpoint = NewAPIEndpoint(conf.APIEndpoint, conf.APIKey)
		if conf.Proxy != nil {
			// we have some kind of proxy configured.
			// make sure our http client uses it
			log.Infof("configuring proxy through host %s", conf.Proxy.Host)
			endpoint.(*APIEndpoint).SetProxy(conf.Proxy)
		}
	} else {
		log.Info("API interface is disabled, flushing to /dev/null instead")
		endpoint = NullEndpoint{}
	}

	return &Writer{
		endpoint: endpoint,

		// small buffer to not block in case we're flushing
		inPayloads: make(chan model.AgentPayload, 1),

		payloadBuffer: make([]*writerPayload, 0, 5),
		serviceBuffer: make(model.ServicesMetadata),

		exit:   make(chan struct{}),
		exitWG: &sync.WaitGroup{},

		conf: conf,
	}
}

// Run starts the writer.
func (w *Writer) Run() {
	w.exitWG.Add(1)
	watchdog.Go(func() {
		w.main()
	})
}

// main is the main loop of the writer goroutine. If buffers payloads and
// services read from input chans and flushes them when necessary.
// NOTE: this currently happens sequentially, but it would not be too
// hard to mutex and parallelize. Not sure it's needed though.
func (w *Writer) main() {
	defer w.exitWG.Done()

	flushTicker := time.NewTicker(time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case p := <-w.inPayloads:
			if p.IsEmpty() {
				continue
			}
			w.payloadBuffer = append(w.payloadBuffer,
				newWriterPayload(p, w.endpoint))
			w.Flush()
		case <-flushTicker.C:
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
	}
}

// Stop stops the main Run loop
func (w *Writer) Stop() {
	close(w.exit)
	w.exitWG.Wait()
}

// FlushServices initiate a flush of the services to the services endpoint
func (w *Writer) FlushServices() {
	w.endpoint.WriteServices(w.serviceBuffer)
}

// Flush actually writes the data in the API
func (w *Writer) Flush() {
	// Start of flushing
	log.Info("Start flushing")

	var wgWriters sync.WaitGroup
	var wgBuffer sync.WaitGroup

	// We try to write all the payloads from w.payloadBuffer to the specified endpoint.
	retry := make(chan *writerPayload)
	nbErrors := uint64(0)
	for _, p := range w.payloadBuffer {
		wgWriters.Add(1)
		go func(p *writerPayload) {
			defer wgWriters.Done()

			// We already tried to flush recently, so there's no point in trying again right now.
			if p.nextFlush.After(time.Now()) {
				retry <- p
				return
			}

			err := p.write()
			if err == nil {
				return // Everything went fine.
			}

			atomic.AddUint64(&nbErrors, 1)
			if _, ok := err.(*APIError); ok {
				now := time.Now()
				// If the payload is too old, we drop it.
				if now.Sub(p.creationDate) > payloadMaxAge {
					statsd.Client.Count("datadog.trace_agent.writer.dropped_payload", int64(1), []string{"reason:too_old"}, 1)
					return
				}

				// Else we'll retry to send it later.
				p.nextFlush = now.Add(payloadResendDelay)
				retry <- p
			}
		}(p)
	}

	// This goroutine collects all payloads to retry later and append them to the buffer.
	var buffer []*writerPayload
	bufSize := 0
	nbDrops := 0
	wgBuffer.Add(1)
	go func() {
		defer wgBuffer.Done()
		for p := range retry {
			if bufSize+p.size <= w.conf.APIPayloadBufferMaxSize {
				buffer = append(buffer, p)
				bufSize += p.size
			} else {
				nbDrops++
			}
		}
	}()

	// We wait for the writers to finish so we can close the channel of payloads to retry.
	wgWriters.Wait()
	close(retry)

	// We then wait for the buffer of payloads to retry to be filled.
	wgBuffer.Wait()
	w.payloadBuffer = buffer

	// Update stats about this flush.
	nbSuccesses := len(w.payloadBuffer) - int(nbErrors)
	if nbSuccesses > 0 {
		log.Infof("nbSuccesses: %d", nbSuccesses)
		statsd.Client.Count("datadog.trace_agent.writer.flush", int64(nbSuccesses), []string{"status:success"}, 1)
	}
	if nbErrors > 0 {
		log.Infof("nbErrors: %d", nbErrors)
		statsd.Client.Count("datadog.trace_agent.writer.flush", int64(nbErrors), []string{"status:error"}, 1)
	}
	if nbDrops > 0 {
		log.Infof("Dropping %d payloads (payload buffer is full)", nbDrops)
		statsd.Client.Count("datadog.trace_agent.writer.dropped_payload", int64(nbDrops), []string{"reason:buffer_full"}, 1)
	}
	if bufSize > 0 {
		log.Infof("bufSize: %d (Max buffer size: %d)", bufSize, w.conf.APIPayloadBufferMaxSize)
		statsd.Client.Gauge("datadog.trace_agent.writer.payload_buffer_size", float64(bufSize), nil, 1)
	}

	log.Info("End of flushing")
}

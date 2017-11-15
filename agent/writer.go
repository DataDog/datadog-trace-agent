package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"sync"
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

// isPayloadBufferingEnabled returns true if payload buffering is enabled or
// false if it is not.
func (w *Writer) isPayloadBufferingEnabled() bool {
	return w.conf.APIPayloadBufferMaxSize > 0
}

// Run starts the writer.
func (w *Writer) Run() {
	w.exitWG.Add(1)
	go func() {
		defer watchdog.LogOnPanic()
		w.main()
	}()
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
				statsd.Client.Count("datadog.trace_agent.services.updated", 1, nil, 1)
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
	// TODO[leo]: batch payloads in same API key

	var payloads []*writerPayload
	now := time.Now()
	bufSize := 0

	bufferPayload := func(p *writerPayload) {
		payloads = append(payloads, p)
		bufSize += p.size
	}

	nbSuccesses := 0
	nbErrors := 0

	for _, p := range w.payloadBuffer {
		if w.isPayloadBufferingEnabled() && p.nextFlush.After(now) {
			// We already tried to flush recently, so there's no
			// point in trying again right now.
			bufferPayload(p)
			continue
		}

		err := p.write()

		if err == nil {
			nbSuccesses++
		} else {
			nbErrors++
		}

		if err == nil || !w.isPayloadBufferingEnabled() {
			continue
		}

		if terr, ok := err.(*apiError); ok {
			// We could not send the payload and this is an API
			// endpoint error, so we can try again later.

			if now.Sub(p.creationDate) > payloadMaxAge {
				// The payload is too old, let's drop it
				statsd.Client.Count("datadog.trace_agent.writer.dropped_payload",
					int64(1), []string{"reason:too_old"}, 1)
				continue
			}

			p.nextFlush = now.Add(payloadResendDelay)

			// Keep this payload in the buffer to try again later,
			// but only with the endpoints that failed.
			p.endpoint = terr.endpoint
			bufferPayload(p)
		}
	}

	if nbSuccesses > 0 {
		statsd.Client.Count("datadog.trace_agent.writer.flush",
			int64(nbSuccesses), []string{"status:success"}, 1)
	}

	if nbErrors > 0 {
		statsd.Client.Count("datadog.trace_agent.writer.flush",
			int64(nbErrors), []string{"status:error"}, 1)
	}

	// Drop payloads to respect the buffer size limit if necessary.
	nbDrops := 0
	for n := 0; n < len(payloads) && bufSize > w.conf.APIPayloadBufferMaxSize; n++ {
		bufSize -= payloads[n].size
		nbDrops++
	}

	if nbDrops > 0 {
		log.Infof("dropping %d payloads (payload buffer full)", nbDrops)
		statsd.Client.Count("datadog.trace_agent.writer.dropped_payload",
			int64(nbDrops), []string{"reason:buffer_full"}, 1)

		payloads = payloads[nbDrops:]
	}

	statsd.Client.Gauge("datadog.trace_agent.writer.payload_buffer_size",
		float64(bufSize), nil, 1)

	w.payloadBuffer = payloads
}

// Flusher provides a method for flushing transactions to a sink
type Flusher interface {
	Flush(*model.SparseAgentPayload) error
}

// LogAgentFlusher flushes transactions to the logs agent
type LogAgentFlusher struct {
	endpoint string
}

// LogAgentPayload wraps the json to the logs agnet
// TODO[aaditya]: probably kill this
type LogAgentPayload struct {
	Message string `json:"message"`
}

// Flush flushes a transaction payload to the logs agent
func (l LogAgentFlusher) Flush(payload *model.SparseAgentPayload) error {
	log.Info("flushing payload to logs agent")
	var buf bytes.Buffer
	for _, t := range payload.Transactions {
		b, err := json.Marshal(t)
		if err == nil {
			buf.Write(b)
			buf.WriteRune('\n')
		}
	}
	log.Info("flushing payload: %s", buf.String())

	logPayload := LogAgentPayload{Message: buf.String()}
	logBytes, err := json.Marshal(logPayload)
	if err != nil {
		log.Errorf("failed to encode transaction payload: %v", err)
	}

	// TODO meter this
	tcpAddr, err := net.ResolveTCPAddr("tcp", l.endpoint)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	defer conn.Close()

	if err != nil {
		log.Errorf("failed to dial tcp %s %s", l.endpoint, err)
	}
	n, err := conn.Write(logBytes)
	_, err = conn.Write([]byte{'\n'})

	if err != nil {
		log.Errorf("failed to write to tcp conn %s", l.endpoint)
	} else {
		log.Infof("wrote %v bytes", n)
	}

	log.Info("Sent payload to logs agent")
	return err
}

// IntakeFlusher flushes to the logs intake
type IntakeFlusher struct {
	endpoint string
}

// Flush flushes a transaction payload to the logs intake
func (i IntakeFlusher) Flush(payload *model.SparseAgentPayload) error {
	log.Info("flushing payload to logs intake")
	bs, err := payload.ToProtobufBytes()
	if err != nil {
		log.Errorf("failed to encode transaction payload: %v", err)
	}

	// TODO meter this
	_, err = http.Post(i.endpoint, "application/octet-stream", bytes.NewReader(bs))
	if err != nil {
		log.Errorf("failed to send transaction payload: %v", err)
	}

	return err
}

// TransactionWriter writes transactions
type TransactionWriter struct {
	Flusher

	in      chan *model.SparseAgentPayload // payloads for root spans
	payload *model.SparseAgentPayload

	exit chan struct{}
}

// NewTransactionWriter creates a new transaction writer with sane defaults
func NewTransactionWriter() *TransactionWriter {
	return &TransactionWriter{
		LogAgentFlusher{"localhost:10520"},
		make(chan *model.SparseAgentPayload, 100),
		nil,
		make(chan struct{}),
	}
}

// Run runs the thing
func (l *TransactionWriter) Run() {
	log.Info("Running transaction writer")
	flushTicker := time.NewTicker(time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case p := <-l.in:
			log.Info("Flushing payload")
			l.Flush(p)
		case <-l.exit:
			return
		}
	}
}

func (tw *TransactionWriter) Add(transaction *model.AnalyzedTransaction) {
	tw.payload.Transactions = append(tw.payload.Transactions, transaction)
}

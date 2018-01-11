package writer

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// ServiceWriter ingests service metadata and flush them to the API.
type ServiceWriter struct {
	endpoint Endpoint

	InServices <-chan model.ServicesMetadata

	serviceBuffer model.ServicesMetadata
	updated       bool

	stats info.ServiceWriterInfo

	exit   chan struct{}
	exitWG *sync.WaitGroup

	conf *config.AgentConfig
}

// NewServiceWriter returns a new writer for services.
func NewServiceWriter(conf *config.AgentConfig, InServices <-chan model.ServicesMetadata) *ServiceWriter {
	var endpoint Endpoint

	if conf.APIEnabled {
		client := NewClient(conf)
		endpoint = NewDatadogEndpoint(client, conf.APIEndpoint, "/api/v0.2/services", conf.APIKey)
	} else {
		log.Info("API interface is disabled, flushing to /dev/null instead")
		endpoint = &NullEndpoint{}
	}

	return &ServiceWriter{
		endpoint: endpoint,

		InServices: InServices,

		serviceBuffer: make(model.ServicesMetadata),

		exit:   make(chan struct{}),
		exitWG: &sync.WaitGroup{},

		conf: conf,
	}
}

// Start starts the writer.
func (w *ServiceWriter) Start() {
	go func() {
		defer watchdog.LogOnPanic()
		w.Run()
	}()
}

// Run runs the main loop of the writer goroutine. If buffers
// services read from input chan and flushes them when necessary.
func (w *ServiceWriter) Run() {
	w.exitWG.Add(1)
	defer w.exitWG.Done()

	// for now, simply flush every x seconds
	flushTicker := time.NewTicker(5 * time.Second)
	defer flushTicker.Stop()

	updateInfoTicker := time.NewTicker(1 * time.Minute)
	defer updateInfoTicker.Stop()

	log.Debug("starting service writer")

	for {
		select {
		case sm := <-w.InServices:
			updated := w.serviceBuffer.Update(sm)
			if updated {
				w.updated = updated
				statsd.Client.Count("datadog.trace_agent.writer.services.updated", 1, nil, 1)
			}
		case <-flushTicker.C:
			w.Flush()
		case <-updateInfoTicker.C:
			go w.updateInfo()
		case <-w.exit:
			log.Info("exiting service writer, flushing all modified services")
			w.Flush()
			return
		}
	}
}

// Stop stops the main Run loop.
func (w *ServiceWriter) Stop() {
	close(w.exit)
	w.exitWG.Wait()
}

// Flush flushes service metadata, if they changed, to the API
func (w *ServiceWriter) Flush() {
	if !w.updated {
		return
	}
	w.updated = false

	serviceBuffer := w.serviceBuffer

	log.Debugf("going to flush updated service metadata, %d services", len(serviceBuffer))
	atomic.StoreInt64(&w.stats.Services, int64(len(serviceBuffer)))

	data, err := model.EncodeServicesPayload(serviceBuffer)
	if err != nil {
		log.Errorf("encoding issue: %v", err)
		return
	}

	headers := map[string]string{
		languageHeaderKey: strings.Join(info.Languages(), "|"),
		"Content-Type":    "application/json",
	}

	atomic.AddInt64(&w.stats.Bytes, int64(len(data)))

	startFlush := time.Now()

	// Send the payload to the endpoint
	err = w.endpoint.Write(data, headers)

	flushTime := time.Since(startFlush)

	// TODO: if error, depending on why, replay later.
	if err != nil {
		atomic.AddInt64(&w.stats.Errors, 1)
		log.Errorf("failed to flush service payload, time:%s, size:%d bytes, error: %s", flushTime, len(data), err)
		return
	}

	log.Infof("flushed service payload to the API, time:%s, size:%d bytes", flushTime, len(data))
	statsd.Client.Gauge("datadog.trace_agent.service_writer.flush_duration", flushTime.Seconds(), nil, 1)
	atomic.AddInt64(&w.stats.Payloads, 1)
}

func (w *ServiceWriter) updateInfo() {
	var swInfo info.ServiceWriterInfo

	// Load counters and reset them for the next flush
	swInfo.Payloads = atomic.SwapInt64(&w.stats.Payloads, 0)
	swInfo.Services = atomic.SwapInt64(&w.stats.Services, 0)
	swInfo.Bytes = atomic.SwapInt64(&w.stats.Bytes, 0)
	swInfo.Errors = atomic.SwapInt64(&w.stats.Errors, 0)

	statsd.Client.Count("datadog.trace_agent.service_writer.payloads", int64(swInfo.Payloads), nil, 1)
	statsd.Client.Gauge("datadog.trace_agent.service_writer.services", float64(swInfo.Services), nil, 1)
	statsd.Client.Count("datadog.trace_agent.service_writer.bytes", int64(swInfo.Bytes), nil, 1)
	statsd.Client.Count("datadog.trace_agent.service_writer.errors", int64(swInfo.Errors), nil, 1)

	info.UpdateServiceWriterInfo(swInfo)
}

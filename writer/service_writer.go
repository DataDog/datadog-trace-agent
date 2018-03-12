package writer

import (
	"strings"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
)

// ServiceWriter ingests service metadata and flush them to the API.
type ServiceWriter struct {
	conf       writerconfig.ServiceWriterConfig
	InServices <-chan model.ServicesMetadata
	stats      info.ServiceWriterInfo

	serviceBuffer model.ServicesMetadata

	BaseWriter
}

// NewServiceWriter returns a new writer for services.
func NewServiceWriter(conf *config.AgentConfig, InServices <-chan model.ServicesMetadata) *ServiceWriter {
	writerConf := conf.ServiceWriterConfig
	log.Infof("Service writer initializing with config: %+v", writerConf)

	return &ServiceWriter{
		conf:          writerConf,
		InServices:    InServices,
		serviceBuffer: model.ServicesMetadata{},
		BaseWriter: *NewBaseWriter(conf, "/api/v0.2/services", func(endpoint Endpoint) PayloadSender {
			senderConf := writerConf.SenderConfig
			return NewCustomQueuablePayloadSender(endpoint, senderConf)
		}),
	}
}

// Start starts the writer.
func (w *ServiceWriter) Start() {
	w.BaseWriter.Start()
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
	flushTicker := time.NewTicker(w.conf.FlushPeriod)
	defer flushTicker.Stop()

	updateInfoTicker := time.NewTicker(w.conf.UpdateInfoPeriod)
	defer updateInfoTicker.Stop()

	log.Debug("starting service writer")

	// Monitor sender for events
	go func() {
		for event := range w.payloadSender.Monitor() {
			if event == nil {
				continue
			}

			switch event := event.(type) {
			case SenderSuccessEvent:
				log.Infof("flushed service payload to the API, time:%s, size:%d bytes", event.SendStats.SendTime,
					len(event.Payload.Bytes))
				w.statsClient.Gauge("datadog.trace_agent.service_writer.flush_duration",
					event.SendStats.SendTime.Seconds(), nil, 1)
				atomic.AddInt64(&w.stats.Payloads, 1)
			case SenderFailureEvent:
				log.Errorf("failed to flush service payload, time:%s, size:%d bytes, error: %s",
					event.SendStats.SendTime, len(event.Payload.Bytes), event.Error)
				atomic.AddInt64(&w.stats.Errors, 1)
			case SenderRetryEvent:
				log.Errorf("retrying flush service payload, retryNum: %d, delay:%s, error: %s",
					event.RetryNum, event.RetryDelay, event.Error)
				atomic.AddInt64(&w.stats.Retries, 1)
			default:
				log.Debugf("don't know how to handle event with type %T", event)
			}
		}
	}()

	// Main loop
	for {
		select {
		case sm := <-w.InServices:
			w.handleServiceMetadata(sm)
		case <-flushTicker.C:
			w.flush()
		case <-updateInfoTicker.C:
			go w.updateInfo()
		case <-w.exit:
			log.Info("exiting service writer, flushing all modified services")
			w.flush()
			return
		}
	}
}

// Stop stops the main Run loop.
func (w *ServiceWriter) Stop() {
	close(w.exit)
	w.exitWG.Wait()
	w.BaseWriter.Stop()
}

func (w *ServiceWriter) handleServiceMetadata(metadata model.ServicesMetadata) {
	w.serviceBuffer.Merge(metadata)
}

func (w *ServiceWriter) flush() {
	// If no services, we can't construct anything
	if len(w.serviceBuffer) == 0 {
		return
	}

	numServices := len(w.serviceBuffer)
	log.Debugf("going to flush updated service metadata, %d services", numServices)
	atomic.StoreInt64(&w.stats.Services, int64(numServices))

	data, err := model.EncodeServicesPayload(w.serviceBuffer)
	if err != nil {
		log.Errorf("error while encoding service payload: %v", err)
		w.serviceBuffer = make(model.ServicesMetadata)
		return
	}

	headers := map[string]string{
		languageHeaderKey: strings.Join(info.Languages(), "|"),
		"Content-Type":    "application/json",
	}

	atomic.AddInt64(&w.stats.Bytes, int64(len(data)))

	payload := NewPayload(data, headers)
	w.payloadSender.Send(payload)

	w.serviceBuffer = make(model.ServicesMetadata)
}

func (w *ServiceWriter) updateInfo() {
	var swInfo info.ServiceWriterInfo

	// Load counters and reset them for the next flush
	swInfo.Payloads = atomic.SwapInt64(&w.stats.Payloads, 0)
	swInfo.Services = atomic.SwapInt64(&w.stats.Services, 0)
	swInfo.Bytes = atomic.SwapInt64(&w.stats.Bytes, 0)
	swInfo.Errors = atomic.SwapInt64(&w.stats.Errors, 0)
	swInfo.Retries = atomic.SwapInt64(&w.stats.Retries, 0)

	w.statsClient.Count("datadog.trace_agent.service_writer.payloads", int64(swInfo.Payloads), nil, 1)
	w.statsClient.Count("datadog.trace_agent.service_writer.services", int64(swInfo.Services), nil, 1)
	w.statsClient.Count("datadog.trace_agent.service_writer.bytes", int64(swInfo.Bytes), nil, 1)
	w.statsClient.Count("datadog.trace_agent.service_writer.retries", int64(swInfo.Retries), nil, 1)
	w.statsClient.Count("datadog.trace_agent.service_writer.errors", int64(swInfo.Errors), nil, 1)

	info.UpdateServiceWriterInfo(swInfo)
}

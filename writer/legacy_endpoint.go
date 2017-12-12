package writer

import (
	"bytes"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

// timeout is the HTTP timeout for POST requests to the Datadog backend
var timeout = 10 * time.Second

// apiError stores the error triggered we can't send data to the endpoint.
// It implements the error interface.
type apiError struct {
	err      error
	endpoint *APIEndpoint
}

func newAPIError(err error, endpoint *APIEndpoint) *apiError {
	return &apiError{err: err, endpoint: endpoint}
}

// Returns the error message
func (ae *apiError) Error() string {
	return fmt.Sprintf("%s: %v", ae.endpoint.url, ae.err)
}

// AgentEndpoint is an interface where we write the data
// that comes out of the agent
type AgentEndpoint interface {
	// Write sends an agent payload which carries all the
	// pre-processed stats/traces
	Write(b model.AgentPayload) (int, error)

	// WriteServices sends updates about the services metadata
	WriteServices(s model.ServicesMetadata)
}

// APIEndpoint implements AgentEndpoint to send data to a
// an endpoint and API key.
type APIEndpoint struct {
	apiKey string
	url    string
	stats  info.EndpointStats
	client *http.Client
}

// NewAPIEndpoint returns a new APIEndpoint from a given config
// of URL (such as https://trace.agent.datadoghq.com) and API
// keys
func NewAPIEndpoint(url, apiKey string) *APIEndpoint {
	if apiKey == "" {
		panic(fmt.Errorf("No API key"))
	}

	ae := APIEndpoint{
		apiKey: apiKey,
		url:    url,
		client: &http.Client{
			Timeout: timeout,
		},
	}
	go func() {
		defer watchdog.LogOnPanic()
		ae.logStats()
	}()
	return &ae
}

// SetProxy updates the http client used by APIEndpoint to report via the given proxy
func (ae *APIEndpoint) SetProxy(settings *config.ProxySettings) {
	proxyPath, err := settings.URL()
	if err != nil {
		log.Errorf("failed to configure proxy: %v", err)
		return
	}
	ae.client = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyPath),
		},
	}
}

// Write will send the serialized payload to the API endpoint.
func (ae *APIEndpoint) Write(p model.AgentPayload) (int, error) {
	startFlush := time.Now()

	// Serialize the payload to send it to the API
	data, err := model.EncodeAgentPayload(&p)
	if err != nil {
		log.Errorf("encoding issue: %v", err)
		return 0, err
	}

	payloadSize := len(data)
	statsd.Client.Count("datadog.trace_agent.writer.payload_bytes", int64(payloadSize), nil, 1)
	atomic.AddInt64(&ae.stats.TracesBytes, int64(payloadSize))
	atomic.AddInt64(&ae.stats.TracesStats, int64(len(p.Stats)))
	atomic.AddInt64(&ae.stats.TracesPayload, 1)

	// Create the request to be sent to the API
	url := ae.url + model.AgentPayloadAPIPath()
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))

	// If the request cannot be created, there is no point in trying again later,
	// it will always yield the same result.
	if err != nil {
		log.Errorf("could not create request for endpoint %s: %v", url, err)
		atomic.AddInt64(&ae.stats.TracesPayloadError, 1)
		return payloadSize, err
	}

	// Set API key in the header and issue the request
	queryParams := req.URL.Query()
	queryParams.Add("api_key", ae.apiKey)
	req.URL.RawQuery = queryParams.Encode()

	model.SetAgentPayloadHeaders(req.Header, p.Extras())
	resp, err := ae.client.Do(req)

	// If the request fails, we'll try again later.
	if err != nil {
		log.Errorf("error when requesting to endpoint %s: %v", url, err)
		atomic.AddInt64(&ae.stats.TracesPayloadError, 1)
		return payloadSize, newAPIError(err, ae)
	}
	defer resp.Body.Close()

	// We check the status code to see if the request has succeeded.
	if resp.StatusCode/100 != 2 {
		err := fmt.Errorf("request to %s responded with %s", url, resp.Status)
		log.Error(err)
		atomic.AddInt64(&ae.stats.TracesPayloadError, 1)

		// Only retry for 5xx (server) errors
		if resp.StatusCode/100 == 5 {
			return payloadSize, newAPIError(err, ae)
		}

		// Does not retry for other errors
		return payloadSize, err
	}

	flushTime := time.Since(startFlush)
	log.Infof("flushed payload to the API, time:%s, size:%d", flushTime, len(data))
	statsd.Client.Gauge("datadog.trace_agent.writer.flush_duration", flushTime.Seconds(), nil, 1)

	// Everything went fine
	return payloadSize, nil
}

// WriteServices writes services to the services endpoint
// This function very loosely logs and returns if any error happens.
// See comment above.
func (ae *APIEndpoint) WriteServices(s model.ServicesMetadata) {
	// Serialize the data to be sent to the API endpoint
	data, err := model.EncodeServicesPayload(s)
	if err != nil {
		log.Errorf("encoding issue: %v", err)
		return
	}

	payloadSize := len(data)
	atomic.AddInt64(&ae.stats.ServicesBytes, int64(payloadSize))
	atomic.AddInt64(&ae.stats.ServicesPayload, 1)

	// Create the request
	url := ae.url + model.ServicesPayloadAPIPath()
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Errorf("could not create request for endpoint %s: %v", url, err)
		atomic.AddInt64(&ae.stats.ServicesPayloadError, 1)
		return
	}

	// Set the header with the API key and issue the request
	queryParams := req.URL.Query()
	queryParams.Add("api_key", ae.apiKey)
	req.URL.RawQuery = queryParams.Encode()
	model.SetServicesPayloadHeaders(req.Header)
	resp, err := ae.client.Do(req)
	if err != nil {
		log.Errorf("error when requesting to endpoint %s: %v", url, err)
		atomic.AddInt64(&ae.stats.ServicesPayloadError, 1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		log.Errorf("request to %s responded with %s", url, resp.Status)
		atomic.AddInt64(&ae.stats.ServicesPayloadError, 1)
		return
	}

	// Everything went fine.
	log.Infof("flushed %d services to the API", len(s))
}

// logStats periodically submits stats about the endpoint to statsd
func (ae *APIEndpoint) logStats() {
	var accStats info.EndpointStats

	for range time.Tick(time.Minute) {
		// Load counters and reset them for the next flush
		accStats.TracesPayload = atomic.SwapInt64(&ae.stats.TracesPayload, 0)
		accStats.TracesPayloadError = atomic.SwapInt64(&ae.stats.TracesPayloadError, 0)
		accStats.TracesBytes = atomic.SwapInt64(&ae.stats.TracesBytes, 0)
		accStats.TracesStats = atomic.SwapInt64(&ae.stats.TracesStats, 0)
		accStats.ServicesPayload = atomic.SwapInt64(&ae.stats.ServicesPayload, 0)
		accStats.ServicesPayloadError = atomic.SwapInt64(&ae.stats.ServicesPayloadError, 0)
		accStats.ServicesBytes = atomic.SwapInt64(&ae.stats.ServicesBytes, 0)

		statsd.Client.Count("datadog.trace_agent.endpoint.traces_payload", int64(accStats.TracesPayload), nil, 1)
		statsd.Client.Count("datadog.trace_agent.endpoint.traces_payload_error", int64(accStats.TracesPayloadError), nil, 1)
		statsd.Client.Count("datadog.trace_agent.endpoint.traces_bytes", int64(accStats.TracesBytes), nil, 1)
		statsd.Client.Count("datadog.trace_agent.endpoint.traces_stats", int64(accStats.TracesStats), nil, 1)
		statsd.Client.Count("datadog.trace_agent.endpoint.services_payload", int64(accStats.ServicesPayload), nil, 1)
		statsd.Client.Count("datadog.trace_agent.endpoint.services_payload_error", int64(accStats.ServicesPayloadError), nil, 1)
		statsd.Client.Count("datadog.trace_agent.endpoint.services_bytes", int64(accStats.ServicesBytes), nil, 1)

		info.UpdateEndpointStats(accStats)
	}
}

// NullAgentEndpoint implements AgentEndpoint, it just logs data
// and drops everything into /dev/null
type NullAgentEndpoint struct{}

// Write just logs and bails
func (ne NullAgentEndpoint) Write(p model.AgentPayload) (int, error) {
	log.Debug("null endpoint: dropping payload, %d traces, %d stats buckets", p.Traces, p.Stats)
	return 0, nil
}

// WriteServices just logs and stops
func (ne NullAgentEndpoint) WriteServices(s model.ServicesMetadata) {
	log.Debugf("null endpoint: dropping services update %v", s)
}

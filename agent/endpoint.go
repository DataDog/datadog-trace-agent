package main

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
	log "github.com/cihub/seelog"
)

// AgentEndpoint is an interface where we write the data
// that comes out of the agent
type AgentEndpoint interface {
	// Write sends an agent payload which carries all the
	// pre-processed stats/traces
	Write(b model.AgentPayload)
	// WriteServices sends updates about the services metadata
	WriteServices(s model.ServicesMetadata)
}

// APIEndpoint implements AgentEndpoint to send data to a
// list of different endpoints and API keys.
// One URL is associated to one API key, hence the two config
// options need to be of the same length.
type APIEndpoint struct {
	apiKeys []string
	urls    []string
}

// NewAPIEndpoint returns a new APIEndpoint from a given config
// of URLs (such as https://trace.agent.datadoghq.com) and API
// keys.
func NewAPIEndpoint(urls, apiKeys []string) APIEndpoint {
	if len(apiKeys) == 0 {
		panic(fmt.Errorf("No API key"))
	}

	if len(urls) != len(apiKeys) {
		panic(fmt.Errorf("APIEndpoint should be initialized with same number of url/api keys"))
	}

	return APIEndpoint{
		apiKeys: apiKeys,
		urls:    urls,
	}
}

// Write writes the bucket to the API collector endpoint.
// Currently, the errors are just logged and we fail silently, keeping
// writing till we have tried everything. This is because currently the
// choice is to just drop the data we cannot write on the floor.
// FIXME?
func (a APIEndpoint) Write(p model.AgentPayload) {
	data, err := model.EncodeAgentPayload(p)
	if err != nil {
		log.Errorf("encoding issue: %v", err)
		return
	}
	statsd.Client.Count("trace_agent.writer.payload_bytes", int64(len(data)), nil, 1)

	for i := range a.urls {
		startFlush := time.Now()

		url := a.urls[i] + model.AgentPayloadAPIPath()
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
		if err != nil {
			log.Errorf("could not create request for endpoint %s: %v", url, err)
			continue
		}

		queryParams := req.URL.Query()
		queryParams.Add("api_key", a.apiKeys[i])
		req.URL.RawQuery = queryParams.Encode()
		model.SetAgentPayloadHeaders(req.Header)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Errorf("error when requesting to endpoint %s: %v", url, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			log.Errorf("request to %s responded with %s", url, resp.Status)
			continue
		}

		flushTime := time.Since(startFlush)
		log.Infof("flushed payload to the API, time:%s, size:%d", flushTime, len(data))
		truncKey := a.apiKeys[i]
		if len(truncKey) > 5 {
			truncKey = truncKey[0:5]
		}
		tags := []string{
			fmt.Sprintf("url:%s", a.urls[i]),
			fmt.Sprintf("apikey:%s", truncKey),
		}
		statsd.Client.Gauge("trace_agent.writer.flush_duration", flushTime.Seconds(), tags, 1)
	}
}

// WriteServices writes services to the services endpoint
// This function very loosely logs and returns if any error happens.
// See comment above.
func (a APIEndpoint) WriteServices(s model.ServicesMetadata) {
	data, err := model.EncodeServicesPayload(s)
	if err != nil {
		log.Errorf("encoding issue: %v", err)
		return
	}

	for i := range a.urls {
		url := a.urls[i] + model.ServicesPayloadAPIPath()
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
		if err != nil {
			log.Errorf("could not create request for endpoint %s: %v", url, err)
			continue
		}

		queryParams := req.URL.Query()
		queryParams.Add("api_key", a.apiKeys[i])
		req.URL.RawQuery = queryParams.Encode()
		model.SetServicesPayloadHeaders(req.Header)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Errorf("error when requesting to endpoint %s: %v", url, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			log.Errorf("request to %s responded with %s", url, resp.Status)
			continue
		}

		log.Infof("flushed %d services to the API", len(s))
	}
}

// NullEndpoint implements AgentEndpoint, it just logs data
// and drops everything into /dev/null
type NullEndpoint struct{}

// Write just logs and bails
func (ne NullEndpoint) Write(p model.AgentPayload) {
	log.Debug("null endpoint: dropping payload, %d traces, %d stats buckets", p.Traces, p.Stats)
}

// WriteServices just logs and stops
func (ne NullEndpoint) WriteServices(s model.ServicesMetadata) {
	log.Debugf("null endpoint: dropping services update %v", s)
}

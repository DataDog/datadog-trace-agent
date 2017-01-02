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

// apiError stores a list of errors triggered when sending data to a
// list of endpoints. The endpoints member contains an api key and url for
// each error.
type apiError struct {
	errs     []error // the errors, one for each endpoint
	endpoint APIEndpoint
}

func newAPIError() *apiError {
	return &apiError{}
}

func (err *apiError) IsEmpty() bool {
	return len(err.errs) == 0
}

func (err *apiError) Append(url, apiKey string, e error) {
	err.errs = append(err.errs, e)
	err.endpoint.urls = append(err.endpoint.urls, url)
	err.endpoint.apiKeys = append(err.endpoint.apiKeys, apiKey)
}

func (err *apiError) Error() string {
	var buf bytes.Buffer

	for i, e := range err.errs {
		if i > 0 {
			buf.WriteString(", ")
		}

		fmt.Fprintf(&buf, "%s: %v", err.endpoint.urls[i], e)
	}

	return buf.String()
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
func (a APIEndpoint) Write(p model.AgentPayload) (int, error) {
	data, err := model.EncodeAgentPayload(p)
	if err != nil {
		log.Errorf("encoding issue: %v", err)
		return 0, err
	}
	payloadSize := len(data)
	statsd.Client.Count("trace_agent.writer.payload_bytes", int64(payloadSize), nil, 1)

	endpointErr := newAPIError()

	for i := range a.urls {
		startFlush := time.Now()

		url := a.urls[i] + model.AgentPayloadAPIPath()
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
		if err != nil {
			// If the request cannot be created, there is no point
			// in trying again later, it will always yield the
			// same result.
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
			endpointErr.Append(a.urls[i], a.apiKeys[i], err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			err := fmt.Errorf("request to %s responded with %s", url, resp.Status)
			log.Error(err)

			// Only retry for 5xx (server) errors; for 4xx errors,
			// something is wrong with the request and there is
			// usually no point in trying again.
			if resp.StatusCode/100 == 5 {
				endpointErr.Append(a.urls[i], a.apiKeys[i], err)
			}

			continue
		}

		flushTime := time.Since(startFlush)
		log.Infof("flushed payload to the API, time:%s, size:%d", flushTime, len(data))
		statsd.Client.Gauge("trace_agent.writer.flush_duration",
			flushTime.Seconds(), nil, 1)
	}

	if endpointErr.IsEmpty() {
		// The payload was sent to all endpoints without any error
		return payloadSize, nil
	}

	return payloadSize, endpointErr
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
func (ne NullEndpoint) Write(p model.AgentPayload) (int, error) {
	log.Debug("null endpoint: dropping payload, %d traces, %d stats buckets", p.Traces, p.Stats)
	return 0, nil
}

// WriteServices just logs and stops
func (ne NullEndpoint) WriteServices(s model.ServicesMetadata) {
	log.Debugf("null endpoint: dropping services update %v", s)
}

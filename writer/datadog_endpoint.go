package writer

import (
	"bytes"
	"fmt"
	"net/http"
)

// DatadogEndpoint sends payloads to Datadog API.
type DatadogEndpoint struct {
	apiKey string
	url    string
	client *http.Client

	path string
}

// NewDatadogEndpoint returns an initialized DatadogEndpoint, from a provided http client and remote endpoint path.
func NewDatadogEndpoint(client *http.Client, url, path, apiKey string) *DatadogEndpoint {
	if apiKey == "" {
		panic(fmt.Errorf("No API key"))
	}

	return &DatadogEndpoint{
		apiKey: apiKey,
		url:    url,
		path:   path,
		client: client,
	}
}

// Write will send the serialized traces payload to the Datadog traces endpoint.
func (e *DatadogEndpoint) Write(payload []byte, headers map[string]string) error {
	// Create the request to be sent to the API
	url := e.url + e.path
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))

	// If the request cannot be created, there is no point in trying again later,
	// it will always yield the same result.
	if err != nil {
		// atomic.AddInt64(&ae.stats.TracesPayloadError, 1)
		return err
	}

	// Set API key in the header and issue the request
	queryParams := req.URL.Query()
	queryParams.Add("api_key", e.apiKey)
	req.URL.RawQuery = queryParams.Encode()

	SetExtraHeaders(req.Header, headers)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "identity")

	resp, err := e.client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// We check the status code to see if the request has succeeded.
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("request to %s responded with %s", url, resp.Status)
	}

	// Everything went fine
	return nil
}

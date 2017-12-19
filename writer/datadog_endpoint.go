package writer

import (
	"bytes"
	"fmt"
	"net/http"
)

const apiHTTPHeaderKey = "DD-Api-Key"

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

	if err != nil {
		return err
	}

	// Set API key in the header and issue the request
	req.Header.Set(apiHTTPHeaderKey, e.apiKey)

	SetExtraHeaders(req.Header, headers)

	resp, err := e.client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// We check the status code to see if the request has succeeded.
	// TODO: define all legit status code and behave accordingly.
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("request to %s responded with %s", url, resp.Status)
	}

	// Everything went fine
	return nil
}

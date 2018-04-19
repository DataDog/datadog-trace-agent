package writer

import (
	"bytes"
	"fmt"
	"net/http"
)

const apiHTTPHeaderKey = "DD-Api-Key"

// StackStateEndpoint sends payloads to StackState API.
type StackStateEndpoint struct {
	apiKey string
	url    string
	client *http.Client

	path string
}

// NewStackStateEndpoint returns an initialized StackStateEndpoint, from a provided http client and remote endpoint path.
func NewStackStateEndpoint(client *http.Client, url, path, apiKey string) *StackStateEndpoint {
	if apiKey == "" {
		panic(fmt.Errorf("No API key"))
	}

	return &StackStateEndpoint{
		apiKey: apiKey,
		url:    url,
		path:   path,
		client: client,
	}
}

// Write will send the serialized traces payload to the StackState traces endpoint.
func (e *StackStateEndpoint) Write(payload *Payload) error {
	// Create the request to be sent to the API
	url := e.url + e.path
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload.Bytes))

	if err != nil {
		return err
	}

	// Set API key in the header and issue the request
	req.Header.Set(apiHTTPHeaderKey, e.apiKey)

	SetExtraHeaders(req.Header, payload.Headers)

	resp, err := e.client.Do(req)

	if err != nil {
		return &RetriableError{
			err:      err,
			endpoint: e,
		}
	}
	defer resp.Body.Close()

	// We check the status code to see if the request has succeeded.
	// TODO: define all legit status code and behave accordingly.
	if resp.StatusCode/100 != 2 {
		err := fmt.Errorf("request to %s responded with %s", url, resp.Status)
		if resp.StatusCode/100 == 5 {
			// 5xx errors are retriable
			return &RetriableError{
				err:      err,
				endpoint: e,
			}
		}

		// All others aren't
		return err
	}

	// Everything went fine
	return nil
}

func (e *StackStateEndpoint) String() string {
	return fmt.Sprintf("DD endpoint(url=%s, path=%s)", e.url, e.path)
}

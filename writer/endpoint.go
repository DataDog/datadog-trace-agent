package writer

import (
	"fmt"

	log "github.com/cihub/seelog"
)

const languageHeaderKey = "X-Datadog-Reported-Languages"

// endpoint is an interface where we send the data from the Agent.
type endpoint interface {
	// Write writes the payload to the endpoint.
	write(payload *payload) error

	// baseURL returns the base URL for this endpoint. e.g. For the URL "https://trace.agent.datadoghq.eu/api/v0.2/traces"
	// it returns "https://trace.agent.datadoghq.eu".
	baseURL() string
}

// nullEndpoint is a void endpoint dropping data.
type nullEndpoint struct{}

// Write of nullEndpoint just drops the payload and log its size.
func (ne *nullEndpoint) write(payload *payload) error {
	log.Debug("null endpoint: dropping payload, size: %d", len(payload.bytes))
	return nil
}

// BaseURL implements Endpoint.
func (ne *nullEndpoint) baseURL() string { return "<nullEndpoint>" }

// retriableError is an endpoint error that signifies that the associated operation can be retried at a later point.
type retriableError struct {
	err      error
	endpoint endpoint
}

// Error returns the error string.
func (re *retriableError) Error() string {
	return fmt.Sprintf("%s: %v", re.endpoint, re.err)
}

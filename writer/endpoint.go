package writer

import (
	"fmt"
	"net/http"

	log "github.com/cihub/seelog"
)

const languageHeaderKey = "X-Datadog-Reported-Languages"

// Endpoint is an interface where we send the data from the Agent.
type Endpoint interface {
	Write(payload *Payload) error
}

// NullEndpoint is a void endpoint dropping data.
type NullEndpoint struct{}

// Write of NullEndpoint just drops the payload and log its size.
func (ne *NullEndpoint) Write(payload *Payload) error {
	log.Debug("null endpoint: dropping payload, size: %d", len(payload.Bytes))
	return nil
}

// SetExtraHeaders appends a header map to HTTP headers.
func SetExtraHeaders(h http.Header, extras map[string]string) {
	for key, value := range extras {
		h.Set(key, value)
	}
}

// RetriableError is an endpoint error that signifies that the associated operation can be retried at a later point.
type RetriableError struct {
	err      error
	endpoint Endpoint
}

// Error returns the error string.
func (re *RetriableError) Error() string {
	return fmt.Sprintf("%s: %v", re.endpoint, re.err)
}

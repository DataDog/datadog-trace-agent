package writer

import (
	"net/http"

	log "github.com/cihub/seelog"
)

const languageHeaderKey = "X-Datadog-Reported-Languages"

// Endpoint is an interface where we send the data from the Agent.
type Endpoint interface {
	Write(payload []byte, headers map[string]string) error
}

// NullEndpoint is a void endpoint dropping data.
type NullEndpoint struct{}

// Write of NullEndpoint just drops the payload and log its size.
func (ne *NullEndpoint) Write(payload []byte, headers map[string]string) error {
	log.Debug("null endpoint: dropping payload, size: %d", len(payload))
	return nil
}

// SetExtraHeaders appends a header map to HTTP headers.
func SetExtraHeaders(h http.Header, extras map[string]string) {
	for key, value := range extras {
		h.Set(key, value)
	}
}

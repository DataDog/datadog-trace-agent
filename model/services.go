package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

// ServicesMetadata is a standard key/val meta map attached to each named service
type ServicesMetadata map[string]map[string]string

// Update compares this metadata blob with the one given in the argument
// if different, update s1 and return true. If equal, return false
func (s1 ServicesMetadata) Update(s2 ServicesMetadata) bool {
	if reflect.DeepEqual(s1, s2) {
		return false
	}

	for s, metas := range s2 {
		s1[s] = metas
	}

	return true
}

// EncodeServicesPayload will return a slice of bytes representing the
// services metadata, this uses the same versioned endpoint that AgentPayload
// uses for serialization.
// Hence watch for GlobalAgentPayloadVersion's value.
func EncodeServicesPayload(sm ServicesMetadata) ([]byte, error) {
	return json.Marshal(sm)
}

// ServicesPayloadAPIPath returns the path to append to the URL to get
// the endpoint for submitting a services metadata payload.
func ServicesPayloadAPIPath() string {
	return fmt.Sprintf("/api/%s/services", GlobalAgentPayloadVersion)
}

// SetServicesPayloadHeaders takes a Header struct and adds the appropriate
// header keys for the API to be able to decode the services metadata.
func SetServicesPayloadHeaders(h http.Header) {
	switch GlobalAgentPayloadVersion {
	case AgentPayloadV01:
		h.Set("Content-Type", "application/json")
	default:
	}
}

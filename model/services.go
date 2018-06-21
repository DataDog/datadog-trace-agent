package model

import (
	"encoding/json"
	"net/http"
)

//go:generate msgp -marshal=false

// ServicesMetadata is a standard key/val meta map attached to each named service
type ServicesMetadata map[string]map[string]string

// AppType is one of the pieces of information embedded in ServiceMetadata
const AppType = "app_type"

// ServiceApp represents the app to which certain integration belongs to
const ServiceApp = "app"

// Merge adds all entries from s2 to s1
func (s1 ServicesMetadata) Merge(s2 ServicesMetadata) {
	for k, v := range s2 {
		s1[k] = v
	}
}

// EncodeServicesPayload will return a slice of bytes representing the
// services metadata, this uses the same versioned endpoint that AgentPayload
// uses for serialization.
// Hence watch for GlobalAgentPayloadVersion's value.
func EncodeServicesPayload(sm ServicesMetadata) ([]byte, error) {
	return json.Marshal(sm)
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

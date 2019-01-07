package pb

import (
	"encoding/json"
)

// ServicesMetadata is a standard key/val meta map attached to each named service
type ServicesMetadata map[string]map[string]string

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

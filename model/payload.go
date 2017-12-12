package model

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// AgentPayload is the main payload to carry data that has been
// pre-processed to the Datadog mothership
type AgentPayload struct {
	HostName string        `json:"hostname"` // the host name that will be resolved by the API
	Env      string        `json:"env"`      // the default environment this agent uses
	Traces   []Trace       `json:"traces"`   // the traces we sampled
	Stats    []StatsBucket `json:"stats"`    // the statistics we pre-computed

	// private
	mu     sync.RWMutex
	extras map[string]string
}

// IsEmpty tells if a payload contains data. If not, it's useless
// to flush it.
func (p *AgentPayload) IsEmpty() bool {
	return len(p.Stats) == 0 && len(p.Traces) == 0
}

// Extras returns this payloads extra metadata fields
func (p *AgentPayload) Extras() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.extras
}

// SetExtra sets the given metadata field on a payload
func (p *AgentPayload) SetExtra(key, val string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.extras == nil {
		p.extras = make(map[string]string)
	}

	p.extras[key] = val
}

// AgentPayloadVersion is the version the agent agrees to with
// the API so that they can encode/decode the data accordingly
type AgentPayloadVersion string

const (
	// AgentPayloadV01 is a simple json'd/gzip'd dump of the payload
	AgentPayloadV01 AgentPayloadVersion = "v0.1"
)

var (
	// GlobalAgentPayloadVersion is a default that will be used
	// in all the AgentPayload method. Override for special cases.
	GlobalAgentPayloadVersion = AgentPayloadV01
)

// EncodeAgentPayload will return a slice of bytes representing the
// payload (according to GlobalAgentPayloadVersion)
func EncodeAgentPayload(p *AgentPayload) ([]byte, error) {
	var b bytes.Buffer
	var err error

	switch GlobalAgentPayloadVersion {
	case AgentPayloadV01:
		gz, err := gzip.NewWriterLevel(&b, gzip.BestSpeed)
		if err != nil {
			return nil, err
		}
		err = json.NewEncoder(gz).Encode(p)
		gz.Close()
	default:
		err = errors.New("unknown payload version")
	}

	return b.Bytes(), err
}

// AgentPayloadAPIPath returns the path (after the first slash) to which
// the payload should be sent to be understood by the API given the
// configured payload version.
func AgentPayloadAPIPath() string {
	return fmt.Sprintf("/api/%s/collector", GlobalAgentPayloadVersion)
}

// SetAgentPayloadHeaders takes a Header struct and adds the appropriate
// header keys for the API to be able to decode the data.
func SetAgentPayloadHeaders(h http.Header, extras map[string]string) {
	switch GlobalAgentPayloadVersion {
	case AgentPayloadV01:
		h.Set("Content-Type", "application/json")
		h.Set("Content-Encoding", "gzip")

		for key, value := range extras {
			h.Set(key, value)
		}
	default:
	}
}

func SetExtraHeaders(h http.Header, extras map[string]string) {
	for key, value := range extras {
		h.Set(key, value)
	}
}

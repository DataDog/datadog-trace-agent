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

// Payload is the main payload to carry data that has been
// pre-processed to the Datadog mothership.
// This is a legacy payload format, used in API v0.1.
type Payload struct {
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
func (p *Payload) IsEmpty() bool {
	return len(p.Stats) == 0 && len(p.Traces) == 0
}

// Extras returns this payloads extra metadata fields
func (p *Payload) Extras() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.extras
}

// SetExtra sets the given metadata field on a payload
func (p *Payload) SetExtra(key, val string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.extras == nil {
		p.extras = make(map[string]string)
	}

	p.extras[key] = val
}

// PayloadVersion is the version the agent agrees to with
// the API so that they can encode/decode the data accordingly
type PayloadVersion string

const (
	// PayloadV01 is a simple json'd/gzip'd dump of the payload
	PayloadV01 PayloadVersion = "v0.1"
)

var (
	// GlobalPayloadVersion is a default that will be used
	// in all the Payload method. Override for special cases.
	GlobalPayloadVersion = PayloadV01
)

// EncodePayload will return a slice of bytes representing the
// payload (according to GlobalPayloadVersion)
func EncodePayload(p *Payload) ([]byte, error) {
	var b bytes.Buffer
	var err error

	switch GlobalPayloadVersion {
	case PayloadV01:
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

// PayloadAPIPath returns the path (after the first slash) to which
// the payload should be sent to be understood by the API given the
// configured payload version.
func PayloadAPIPath() string {
	return fmt.Sprintf("/api/%s/collector", GlobalPayloadVersion)
}

// SetPayloadHeaders takes a Header struct and adds the appropriate
// header keys for the API to be able to decode the data.
func SetPayloadHeaders(h http.Header, extras map[string]string) {
	switch GlobalPayloadVersion {
	case PayloadV01:
		h.Set("Content-Type", "application/json")
		h.Set("Content-Encoding", "gzip")

		for key, value := range extras {
			h.Set(key, value)
		}
	default:
	}
}

package config

import (
	"encoding/gob"
	"os"
)

type analyzedServices map[string]float64

// ServerConfig contains configuration options sent down by the Datadog API
type ServerConfig struct {
	ModifyIndex int64 `json:modify_index,omitempty`

	AnalyzedServices analyzedServices `json:analyzed_services,omitempty`
}

// NewServerConfigFromFile initializes ServerConfig from a state file on disk
func NewServerConfigFromFile(file string) (*ServerConfig, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	var s ServerConfig
	decoder := gob.NewDecoder(f)
	err = decoder.Decode(&s)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

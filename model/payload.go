package model

// AgentPayload is the payload sent to the API of mothership with raw traces
type AgentPayload struct {
	HostName string        `json:"hostname"`
	Traces   []Trace       `json:"traces"`
	Stats    []StatsBucket `json:"stats"`
}

// IsEmpty tells if the payload entirely empty, with no need to flush it
func (p *AgentPayload) IsEmpty() bool {
	return len(p.Stats) == 0 && len(p.Traces) == 0
}

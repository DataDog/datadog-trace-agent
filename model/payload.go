package model

// AgentPayload is the payload sent to the API of mothership with raw traces
type AgentPayload struct {
	HostName string              `json:"hostname"`
	Spans    []Span              `json:"spans"`
	Stats    []StatsBucket       `json:"stats"`
	Graph    map[string][]uint64 `json:"graph"`
}

// IsEmpty tells if the payload is empty (and don't need to be sent)
func (p *AgentPayload) IsEmpty() bool {
	return len(p.Spans) == 0 && len(p.Stats) == 0 // Use p.Stats.IsEmpty()?
}

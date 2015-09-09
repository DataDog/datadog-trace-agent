package model

// AgentPayload is the payload sent to the API of mothership with raw traces
type AgentPayload struct {
	APIKey string      `json:"api_key"`
	Spans  []Span      `json:"spans"`
	Stats  StatsBucket `json:"stats"`
}

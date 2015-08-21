package model

// Payload is the payload sent to the API of mothership with raw traces
type Payload struct {
	APIKey string       `json:"api_key"`
	Spans  []*Span      `json:"spans"`
	Stats  *StatsBucket `json:"stats"`
}

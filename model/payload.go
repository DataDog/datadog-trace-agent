package model

// SpanPayload is the payload sent to the API of mothership with raw traces
type SpanPayload struct {
	APIKey string `json:"api_key"`
	Spans  []Span `json:"spans"`
}

// StatsPayload is the payload sent to the API of mothership for pre-digested stats, that will be further aggregated in the backend
type StatsPayload struct {
	APIKey string        `json:"api_key"`
	Stats  []StatsBucket `json:"stats"`
}

package model

// SpanPayload is the payload sent to the API of mothership with raw traces
type SpanPayload struct {
	APIKey string       `json:"api_key"`
	Spans  []Span       `json:"spans"`
	Stats  *StatsBucket `json:"stats"`
}

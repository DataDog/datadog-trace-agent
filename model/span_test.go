package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpanNormalizeFail(t *testing.T) {
	assert := assert.New(t)
	spans := []Span{
		Span{},
		// service not set
		Span{Name: "pylons.controller", Resource: "index", SpanID: 42, Start: 0, Duration: 10},
		// name not set
		Span{Service: "pylons", Resource: "index", SpanID: 42, Start: 0, Duration: 10},
		// resource not set
		Span{Service: "pylons", Name: "pylons.controller", SpanID: 42, Start: 0, Duration: 10},
	}

	for _, s := range spans {
		assert.NotNil(s.Normalize())
	}
}

func TestSpanNormalizeSoftFail(t *testing.T) {
	assert := assert.New(t)

	// sets trace ID if missing
	s1 := Span{Service: "pylons", Name: "pylons.controller", Resource: "index", SpanID: 42, Start: 42, Duration: 10}
	assert.Nil(s1.Normalize())
	assert.NotEqual(0, s1.TraceID)

	// sets span ID if missing
	s2 := Span{Service: "pylons", Name: "pylons.controller", Resource: "index", TraceID: 42, Start: 42, Duration: 10}
	assert.Nil(s2.Normalize())
	assert.NotEqual(0, s2.SpanID)

	// sets start as Now() if missing
	s3 := Span{Service: "pylons", Name: "pylons.controller", Resource: "index", TraceID: 42, Duration: 10}
	assert.Nil(s3.Normalize())
	now := Now()
	// 10s range
	assert.True((s3.Start > now-5e9) && (s3.Start < now+5e9), "now: %d, val: %d", now, s3.Start)
}

var testSpan = Span{
	Service:  "pylons",
	Name:     "pylons.controller",
	Resource: "index",
	TraceID:  8324092384029,
	SpanID:   3049283402,
	Start:    Now(),
	Duration: 1e9,
	Error:    42,
	Meta: map[string]string{
		"user":          "leo",
		"http.x-source": "89.89.89.89",
		"auth":          "valid",
	},
	Metrics: map[string]int64{
		"request_size":  3242379,
		"response_size": 284302948,
		"lb_latency":    232,
	},
	ParentID: 230948230,
	Type:     "sql",
}

func TestSpanNormalizeFullSpan(t *testing.T) {
	assert := assert.New(t)
	tcopy := Span(testSpan)
	assert.Nil(tcopy.Normalize())
	assert.Equal(testSpan, tcopy)
}

func TestSpanString(t *testing.T) {
	assert := assert.New(t)
	assert.NotEqual("", testSpan.String())
}

func TestSpanFlushMarker(t *testing.T) {
	assert := assert.New(t)
	s := NewFlushMarker()
	assert.True(s.IsFlushMarker())
}

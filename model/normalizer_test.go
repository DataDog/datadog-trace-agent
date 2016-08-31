package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeOK(t *testing.T) {
	s := Span(testSpan)
	assert.Nil(t, s.Normalize())
}

func TestNormalizeServicePassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Service
	s.Normalize()
	assert.Equal(t, before, s.Service)
}

func TestNormalizeEmptyService(t *testing.T) {
	s := Span(testSpan)
	s.Service = ""
	assert.NotNil(t, s.Normalize())
}

func TestNormalizeLongService(t *testing.T) {
	s := Span(testSpan)
	s.Service = strings.Repeat("CAMEMBERT", 100)
	assert.NotNil(t, s.Normalize())
}

func TestNormalizeNamePassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Name
	s.Normalize()
	assert.Equal(t, before, s.Name)
}

func TestNormalizeEmptyName(t *testing.T) {
	s := Span(testSpan)
	s.Name = ""
	assert.NotNil(t, s.Normalize())
}

func TestNormalizeLongName(t *testing.T) {
	s := Span(testSpan)
	s.Name = strings.Repeat("CAMEMBERT", 100)
	assert.NotNil(t, s.Normalize())
}

func TestNormalizeName(t *testing.T) {
	expNames := map[string]string{
		"pylons.controller": "pylons.controller",
		"trace-api.request": "trace_api.request",
	}

	s := Span(testSpan)
	for name, expName := range expNames {
		s.Name = name
		assert.Nil(t, s.Normalize())
		assert.Equal(t, expName, s.Name)
	}
}

func TestNormalizeResourcePassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Resource
	s.Normalize()
	assert.Equal(t, before, s.Resource)
}

func TestNormalizeEmptyResource(t *testing.T) {
	s := Span(testSpan)
	s.Resource = ""
	assert.NotNil(t, s.Normalize())
}

func TestNormalizeLongResource(t *testing.T) {
	s := Span(testSpan)
	s.Resource = strings.Repeat("SELECT ", 5000)
	assert.Nil(t, s.Normalize())
	assert.Equal(t, 5000, len(s.Resource))
}

func TestNormalizeTraceIDPassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.TraceID
	s.Normalize()
	assert.Equal(t, before, s.TraceID)
}

func TestNormalizeNoTraceID(t *testing.T) {
	s := Span(testSpan)
	s.TraceID = 0
	s.Normalize()
	assert.NotEqual(t, 0, s.TraceID)
}

func TestNormalizeSpanIDPassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.SpanID
	s.Normalize()
	assert.Equal(t, before, s.SpanID)
}

func TestNormalizeNoSpanID(t *testing.T) {
	s := Span(testSpan)
	s.SpanID = 0
	s.Normalize()
	assert.NotEqual(t, 0, s.SpanID)
}

func TestNormalizeStartPassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Start
	s.Normalize()
	assert.Equal(t, before, s.Start)
}

func TestNormalizeStartTooSmall(t *testing.T) {
	s := Span(testSpan)
	s.Start = 42
	assert.NotNil(t, s.Normalize())
}

func TestNormalizeDurationPassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Duration
	s.Normalize()
	assert.Equal(t, before, s.Duration)
}

func TestNormalizeEmptyDuration(t *testing.T) {
	s := Span(testSpan)
	s.Duration = 0
	assert.NotNil(t, s.Normalize())
}

func TestNormalizeErrorPassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Error
	s.Normalize()
	assert.Equal(t, before, s.Error)
}

func TestNormalizeMetricsPassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Metrics
	s.Normalize()
	assert.Equal(t, before, s.Metrics)
}

func TestNormalizeMetricsKeyTooLong(t *testing.T) {
	s := Span(testSpan)
	key := strings.Repeat("TOOLONG", 1000)
	s.Metrics[key] = 42
	assert.Nil(t, s.Normalize())
	for k := range s.Metrics {
		assert.True(t, len(k) < MaxMetricsKeyLen+4)
	}
}

func TestNormalizeMetaPassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Meta
	s.Normalize()
	assert.Equal(t, before, s.Meta)
}

func TestNormalizeMetaKeyTooLong(t *testing.T) {
	s := Span(testSpan)
	key := strings.Repeat("TOOLONG", 1000)
	s.Meta[key] = "foo"
	assert.Nil(t, s.Normalize())
	for k := range s.Meta {
		assert.True(t, len(k) < MaxMetaKeyLen+4)
	}
}

func TestNormalizeMetaValueTooLong(t *testing.T) {
	s := Span(testSpan)
	val := strings.Repeat("TOOLONG", 5000)
	s.Meta["foo"] = val
	assert.Nil(t, s.Normalize())
	for _, v := range s.Meta {
		assert.True(t, len(v) < MaxMetaValLen+4)
	}
}

func TestNormalizeParentIDPassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.ParentID
	s.Normalize()
	assert.Equal(t, before, s.ParentID)
}

func TestNormalizeTypePassThru(t *testing.T) {
	s := Span(testSpan)
	before := s.Type
	s.Normalize()
	assert.Equal(t, before, s.Type)
}

func TestNormalizeTypeTooLong(t *testing.T) {
	s := Span(testSpan)
	s.Type = strings.Repeat("sql", 1000)
	s.Normalize()
	assert.NotNil(t, s.Normalize())
}

func TestNormalizeServiceTag(t *testing.T) {
	s := Span(testSpan)
	s.Service = "retargeting(api-Staging "
	s.Normalize()
	assert.Equal(t, "retargeting_api-staging", s.Service)
}

func TestNormalizeInequalityRootSpan(t *testing.T) {
	s := Span(testSpan)
	s.ParentID = 42
	s.TraceID = 42
	s.SpanID = 42
	beforeTraceID := s.TraceID
	beforeSpanID := s.SpanID
	s.Normalize()
	assert.Equal(t, uint64(0), s.ParentID)
	assert.Equal(t, beforeTraceID, s.TraceID)
	assert.Equal(t, beforeSpanID, s.SpanID)
}

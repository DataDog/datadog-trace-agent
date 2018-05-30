package model

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeOK(t *testing.T) {
	s := testSpan()
	assert.NoError(t, s.Normalize())
}

func TestNormalizeServicePassThru(t *testing.T) {
	s := testSpan()
	before := s.Service
	s.Normalize()
	assert.Equal(t, before, s.Service)
}

func TestNormalizeEmptyService(t *testing.T) {
	s := testSpan()
	s.Service = ""
	assert.Error(t, s.Normalize())
}

func TestNormalizeLongService(t *testing.T) {
	s := testSpan()
	s.Service = strings.Repeat("CAMEMBERT", 100)
	assert.Error(t, s.Normalize())
}

func TestNormalizeNamePassThru(t *testing.T) {
	s := testSpan()
	before := s.Name
	s.Normalize()
	assert.Equal(t, before, s.Name)
}

func TestNormalizeEmptyName(t *testing.T) {
	s := testSpan()
	s.Name = ""
	assert.Error(t, s.Normalize())
}

func TestNormalizeLongName(t *testing.T) {
	s := testSpan()
	s.Name = strings.Repeat("CAMEMBERT", 100)
	assert.Error(t, s.Normalize())
}

func TestNormalizeName(t *testing.T) {
	expNames := map[string]string{
		"pylons.controller": "pylons.controller",
		"trace-api.request": "trace_api.request",
	}

	s := testSpan()
	for name, expName := range expNames {
		s.Name = name
		assert.NoError(t, s.Normalize())
		assert.Equal(t, expName, s.Name)
	}
}

func TestNormalizeNameFailure(t *testing.T) {
	invalidNames := []string{
		"",   // Empty.
		"/",  // No alphanumerics.
		"//", // Still no alphanumerics.
		strings.Repeat("x", MaxNameLen+1), // Too long.
	}
	s := testSpan()
	for _, v := range invalidNames {
		s.Name = v
		assert.Error(t, s.Normalize())
	}
}

func TestNormalizeResourcePassThru(t *testing.T) {
	s := testSpan()
	before := s.Resource
	s.Normalize()
	assert.Equal(t, before, s.Resource)
}

func TestNormalizeEmptyResource(t *testing.T) {
	s := testSpan()
	s.Resource = ""
	assert.Error(t, s.Normalize())
}

func TestNormalizeTraceIDPassThru(t *testing.T) {
	s := testSpan()
	before := s.TraceID
	s.Normalize()
	assert.Equal(t, before, s.TraceID)
}

func TestNormalizeNoTraceID(t *testing.T) {
	s := testSpan()
	s.TraceID = 0
	s.Normalize()
	assert.NotEqual(t, 0, s.TraceID)
}

func TestNormalizeSpanIDPassThru(t *testing.T) {
	s := testSpan()
	before := s.SpanID
	s.Normalize()
	assert.Equal(t, before, s.SpanID)
}

func TestNormalizeNoSpanID(t *testing.T) {
	s := testSpan()
	s.SpanID = 0
	s.Normalize()
	assert.NotEqual(t, 0, s.SpanID)
}

func TestNormalizeStartPassThru(t *testing.T) {
	s := testSpan()
	before := s.Start
	s.Normalize()
	assert.Equal(t, before, s.Start)
}

func TestNormalizeStartTooSmall(t *testing.T) {
	s := testSpan()
	s.Start = 42
	assert.Error(t, s.Normalize())
}

func TestNormalizeStartTooLarge(t *testing.T) {
	s := testSpan()
	s.Start = time.Now().Add(15 * time.Minute).UnixNano()
	assert.Error(t, s.Normalize())
}

func TestNormalizeDurationPassThru(t *testing.T) {
	s := testSpan()
	before := s.Duration
	s.Normalize()
	assert.Equal(t, before, s.Duration)
}

func TestNormalizeEmptyDuration(t *testing.T) {
	s := testSpan()
	s.Duration = 0
	assert.Error(t, s.Normalize())
}

func TestNormalizeNegativeDuration(t *testing.T) {
	s := testSpan()
	s.Duration = -50
	assert.Error(t, s.Normalize())
}

func TestNormalizeErrorPassThru(t *testing.T) {
	s := testSpan()
	before := s.Error
	s.Normalize()
	assert.Equal(t, before, s.Error)
}

func TestNormalizeMetricsPassThru(t *testing.T) {
	s := testSpan()
	before := s.Metrics
	s.Normalize()
	assert.Equal(t, before, s.Metrics)
}

func TestNormalizeMetaPassThru(t *testing.T) {
	s := testSpan()
	before := s.Meta
	s.Normalize()
	assert.Equal(t, before, s.Meta)
}

func TestNormalizeParentIDPassThru(t *testing.T) {
	s := testSpan()
	before := s.ParentID
	s.Normalize()
	assert.Equal(t, before, s.ParentID)
}

func TestNormalizeTypePassThru(t *testing.T) {
	s := testSpan()
	before := s.Type
	s.Normalize()
	assert.Equal(t, before, s.Type)
}

func TestNormalizeTypeTooLong(t *testing.T) {
	s := testSpan()
	s.Type = strings.Repeat("sql", 1000)
	s.Normalize()
	assert.Error(t, s.Normalize())
}

func TestNormalizeServiceTag(t *testing.T) {
	s := testSpan()
	s.Service = "retargeting(api-Staging "
	s.Normalize()
	assert.Equal(t, "retargeting_api-staging", s.Service)
}

func TestNormalizeEnv(t *testing.T) {
	s := testSpan()
	s.Meta["env"] = "DEVELOPMENT"
	s.Normalize()
	assert.Equal(t, "development", s.Meta["env"])
}

func TestSpecialZipkinRootSpan(t *testing.T) {
	s := testSpan()
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

func TestNormalizeTraceEmpty(t *testing.T) {
	trace := Trace{}

	_, err := NormalizeTrace(trace)
	assert.Error(t, err)
}

func TestNormalizeTraceTraceIdMismatch(t *testing.T) {
	span1 := testSpan()
	span1.TraceID = 1

	span2 := testSpan()
	span2.TraceID = 2

	trace := Trace{span1, span2}

	_, err := NormalizeTrace(trace)
	assert.Error(t, err)
}

func TestNormalizeTraceInvalidSpan(t *testing.T) {
	span1 := testSpan()

	span2 := testSpan()
	span2.Name = "" // invalid

	trace := Trace{span1, span2}

	_, err := NormalizeTrace(trace)
	assert.Error(t, err)
}

func TestNormalizeTraceDuplicateSpanID(t *testing.T) {
	span1 := testSpan()
	span2 := testSpan()
	span2.SpanID = span1.SpanID

	trace := Trace{span1, span2}

	_, err := NormalizeTrace(trace)
	assert.Error(t, err)
}

func TestNormalizeTrace(t *testing.T) {
	span1 := testSpan()

	span2 := testSpan()
	span2.SpanID++

	trace := Trace{span1, span2}

	_, err := NormalizeTrace(trace)
	assert.NoError(t, err)
}

func TestIsValidStatusCode(t *testing.T) {
	assert := assert.New(t)
	assert.True(isValidStatusCode("100"))
	assert.True(isValidStatusCode("599"))
	assert.False(isValidStatusCode("99"))
	assert.False(isValidStatusCode("600"))
	assert.False(isValidStatusCode("Invalid status code"))
}

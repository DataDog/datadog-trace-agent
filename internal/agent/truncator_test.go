package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateResourcePassThru(t *testing.T) {
	s := testSpan()
	before := s.Resource
	s.Truncate()
	assert.Equal(t, before, s.Resource)
}

func TestTruncateLongResource(t *testing.T) {
	s := testSpan()
	s.Resource = strings.Repeat("TOOLONG", 5000)
	s.Truncate()
	assert.Equal(t, 5000, len(s.Resource))
}

func TestTruncateMetricsPassThru(t *testing.T) {
	s := testSpan()
	before := s.Metrics
	s.Truncate()
	assert.Equal(t, before, s.Metrics)
}

func TestTruncateMetricsKeyTooLong(t *testing.T) {
	s := testSpan()
	key := strings.Repeat("TOOLONG", 1000)
	s.Metrics[key] = 42
	s.Truncate()
	for k := range s.Metrics {
		assert.True(t, len(k) < MaxMetricsKeyLen+4)
	}
}

func TestTruncateMetaPassThru(t *testing.T) {
	s := testSpan()
	before := s.Meta
	s.Truncate()
	assert.Equal(t, before, s.Meta)
}

func TestTruncateMetaKeyTooLong(t *testing.T) {
	s := testSpan()
	key := strings.Repeat("TOOLONG", 1000)
	s.Meta[key] = "foo"
	s.Truncate()
	for k := range s.Meta {
		assert.True(t, len(k) < MaxMetaKeyLen+4)
	}
}

func TestTruncateMetaValueTooLong(t *testing.T) {
	s := testSpan()
	val := strings.Repeat("TOOLONG", 5000)
	s.Meta["foo"] = val
	s.Truncate()
	for _, v := range s.Meta {
		assert.True(t, len(v) < MaxMetaValLen+4)
	}
}

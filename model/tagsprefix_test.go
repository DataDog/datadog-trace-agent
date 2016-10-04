package model

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWithTracePrefix(t *testing.T) {
	eg := [][]string{
		{"name", "trace.name"},
		{"trace.name", "trace.name"},
		{"foo", "trace.foo"},
		{"trace.foo", "trace.foo"},
		{"trace.trace.foo", "trace.foo"},
		{".foo", "trace..foo"},
		{"foo.", "trace.foo."},
		{"foo.trace", "trace.foo.trace"},
	}

	for _, v := range eg {
		assert.Equal(t, v[1], WithTracePrefix(v[0]))
	}
}

func TestWithoutTracePrefix(t *testing.T) {
	eg := [][]string{
		{"name", "name"},
		{"trace.name", "name"},
		{"foo", "foo"},
		{"trace.foo", "foo"},
		{"trace.trace.foo", "foo"},
		{".foo", ".foo"},
		{"foo.", "foo."},
		{"foo.trace", "foo.trace"},
	}

	for _, v := range eg {
		assert.Equal(t, v[1], WithoutTracePrefix(v[0]))
	}
}

func BenchmarkWithTracePrefixSpecific(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = IsTraceSpecific("name")
	}
}

func BenchmarkWithTracePrefixGeneric(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = WithTracePrefix("bar")
	}
}

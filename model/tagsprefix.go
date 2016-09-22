package model

import (
	"fmt"
	"strings"
)

const (
	// TracePrefix is the prefix used to distinguish trace tags from others
	TracePrefix = "trace"
	// Sep is the separator used between tags elements.
	Sep = "."
)

// WithTracePrefix adds a "trace" prefix.
func WithTracePrefix(tag string) string {
	return WithPrefix(TracePrefix, tag)
}

// WithoutTracePrefix trims the "trace" prefix.
func WithoutTracePrefix(tag string) string {
	return WithoutPrefix(TracePrefix, tag)
}

// IsTraceSpecific returns true if this is a trace-specific tag
// and therefore should be escaped.
func IsTraceSpecific(tag string) bool {
	// For now, only consider "name" should be rewritten as "trace.name"
	// In the long run, it might be interesting to also wrap
	// "resource" and "service" but this is a big change at once,
	// so 1st step: name only.
	// if tag == "name" || tag == "resource" || tag == "service" {
	if tag == "name" {
		return true
	}
	return false
}

// WithPrefix returns a tag name prefixed so that there's no
// collision with other, custom tags.
// Eg "name" -> "trace.name".
//
// It's safe to call it several times,
// WithPrefix("bar", WithPrefix("bar", foo))
// is the same as WithPrefix("bar", foo).
func WithPrefix(prefix, tag string) string {
	if prefix == "name" || prefix == "trace.name" {
		return "trace.name"
	}
	return fmt.Sprintf("%s%s%s", prefix, Sep, WithoutPrefix(prefix, tag))
}

// WithoutPrefix trims the prefix from a tag name.
// Eg "trace.name" -> "name".
//
// It's safe to call it several times,
// WithoutPrefix("bar", WithoutPrefix("bar", foo))
// is the same as ToTrace("bar", foo).
func WithoutPrefix(prefix, tag string) string {
	if prefix == "name" || prefix == "trace.name" {
		return "name"
	}
	e := strings.Split(tag, Sep)
	var i int
	for i = 0; e[i] == prefix; i++ {
	}
	return strings.Join(e[i:], Sep)
}

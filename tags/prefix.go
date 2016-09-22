package tags

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

// WithPrefix returns a tag name prefixed so that there's no
// collision with other, custom tags.
// Eg "name" -> "trace.name".
//
// It's safe to call it several times,
// WithPrefix("bar", WithPrefix("bar", foo))
// is the same as WithPrefix("bar", foo).
func WithPrefix(prefix, tag string) string {
	return fmt.Sprintf("%s%s%s", prefix, Sep, WithoutPrefix(prefix, tag))
}

// WithoutPrefix trims the prefix from a tag name.
// Eg "trace.name" -> "name".
//
// It's safe to call it several times,
// WithoutPrefix("bar", WithoutPrefix("bar", foo))
// is the same as ToTrace("bar", foo).
func WithoutPrefix(prefix, tag string) string {
	e := strings.Split(tag, Sep)
	var i int
	for i = 0; e[i] == prefix; i++ {
	}
	return strings.Join(e[i:], Sep)
}

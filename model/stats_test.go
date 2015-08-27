package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountHits(t *testing.T) {
	assert := assert.New(t)

	tags := NewTagsFromString("version:34.42,resource:/dash/list,service:dogweb")
	c := NewCount(HITS, tags)

	// Our fake span
	s := Span{TraceID: RandomID(), SpanID: RandomID(), Start: Now(), Duration: 1.0}
	c = c.Add(s)

	assert.Equal(HITS, c.Name)
	assert.Equal(1, c.Value)
	assert.Equal(
		TagSet{
			Tag{Name: "version", Value: "34.42"},
			Tag{Name: "resource", Value: "/dash/list"},
			Tag{Name: "service", Value: "dogweb"},
		},
		c.TagSet,
	)
}

func TestCountErrors(t *testing.T) {
	assert := assert.New(t)

	tags := NewTagsFromString("version:34.42,resource:/dash/list,service:dogweb")
	c := NewCount(ERRORS, tags)

	// Our fake span
	s := Span{TraceID: RandomID(), SpanID: RandomID(), Start: Now(), Duration: 1.0}
	c = c.Add(s)

	assert.Equal(ERRORS, c.Name)
	assert.Equal(1, c.Value)
	assert.Equal(
		TagSet{
			Tag{Name: "version", Value: "34.42"},
			Tag{Name: "resource", Value: "/dash/list"},
			Tag{Name: "service", Value: "dogweb"},
		},
		c.TagSet,
	)
}

func TestCountTimes(t *testing.T) {
	assert := assert.New(t)

	tags := NewTagsFromString("version:34.42,resource:/dash/list,service:dogweb")
	c := NewCount(DURATION, tags)

	// Our fake span
	s := Span{TraceID: RandomID(), SpanID: RandomID(), Start: Now(), Duration: 1e6}
	c = c.Add(s)

	assert.Equal(DURATION, c.Name)
	assert.Equal(1000000, c.Value)
	assert.Equal(
		TagSet{
			Tag{Name: "version", Value: "34.42"},
			Tag{Name: "resource", Value: "/dash/list"},
			Tag{Name: "service", Value: "dogweb"},
		},
		c.TagSet,
	)
}

func TestCountBad(t *testing.T) {
	assert := assert.New(t)
	c := NewCount("raclette", TagSet{})
	s := Span{TraceID: RandomID(), SpanID: RandomID(), Start: Now(), Duration: 1e6}
	assert.Panics(func() { c.Add(s) })
}

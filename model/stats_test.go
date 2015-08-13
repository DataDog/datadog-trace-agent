package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountHits(t *testing.T) {
	assert := assert.New(t)

	tags := NewTagsFromString("version:34.42,resource:/dash/list,service:dogweb")
	c := NewCount(HITS, &tags, 0.01)

	// Our fake span
	s := Span{TraceID: NewTID(), SpanID: NewSID(), Start: Now(), Duration: 1.0}
	c.Add(&s)

	assert.Equal(HITS, c.Name)
	assert.Equal(1, c.Value)
	assert.Equal(
		[]Tag{
			Tag{Name: "version", Value: "34.42"},
			Tag{Name: "resource", Value: "/dash/list"},
			Tag{Name: "service", Value: "dogweb"},
		},
		c.Tags,
	)
	assert.Nil(c.Distribution)
}

func TestCountErrors(t *testing.T) {
	assert := assert.New(t)

	tags := NewTagsFromString("version:34.42,resource:/dash/list,service:dogweb")
	c := NewCount(ERRORS, &tags, 0.01)

	// Our fake span
	s := Span{TraceID: NewTID(), SpanID: NewSID(), Start: Now(), Duration: 1.0}
	c.Add(&s)

	assert.Equal(ERRORS, c.Name)
	assert.Equal(1, c.Value)
	assert.Equal(
		[]Tag{
			Tag{Name: "version", Value: "34.42"},
			Tag{Name: "resource", Value: "/dash/list"},
			Tag{Name: "service", Value: "dogweb"},
		},
		c.Tags,
	)
	assert.Nil(c.Distribution)
}

func TestCountTimes(t *testing.T) {
	assert := assert.New(t)

	tags := NewTagsFromString("version:34.42,resource:/dash/list,service:dogweb")
	c := NewCount(TIMES, &tags, 0.01)

	// Our fake span
	s := Span{TraceID: NewTID(), SpanID: NewSID(), Start: Now(), Duration: 1.0}
	c.Add(&s)

	assert.Equal(TIMES, c.Name)
	assert.Equal(1, c.Value)
	assert.Equal(
		[]Tag{
			Tag{Name: "version", Value: "34.42"},
			Tag{Name: "resource", Value: "/dash/list"},
			Tag{Name: "service", Value: "dogweb"},
		},
		c.Tags,
	)
	assert.NotNil(c.Distribution)
	d, ok := c.Distribution.(*GKDistro)

	assert.True(ok)
	assert.Equal(1, d.n)
}

func TestCountBad(t *testing.T) {
	assert := assert.New(t)
	var tags []Tag
	c := NewCount("raclette", &tags, 0.1)
	s := Span{TraceID: NewTID(), SpanID: NewSID(), Start: Now(), Duration: 1.0}
	assert.Panics(func() { c.Add(&s) })
}

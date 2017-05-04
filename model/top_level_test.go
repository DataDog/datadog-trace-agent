package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopLevel(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web"},
		Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "mcnulty", Type: "sql"},
		Span{TraceID: 1, SpanID: 3, ParentID: 2, Service: "master-db", Type: "sql"},
		Span{TraceID: 1, SpanID: 4, ParentID: 1, Service: "redis", Type: "redis"},
		Span{TraceID: 1, SpanID: 5, ParentID: 1, Service: "mcnulty", Type: ""},
	}

	tr.ComputeTopLevel()

	assert.True(tr[0].TopLevel, "root span should be top-level")
	assert.False(tr[1].TopLevel, "main service, and not a root span, not top-level")
	assert.True(tr[2].TopLevel, "only 1 span for this service, should be top-level")
	assert.True(tr[3].TopLevel, "only 1 span for this service, should be top-level")
	assert.False(tr[4].TopLevel, "yet another sup span, not top-level")
}

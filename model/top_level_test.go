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

	assert.Equal("true", tr[0].Meta["_top_level"], "root span should be top-level")
	assert.Nil(tr[1].Meta, "main service, and not a root span, not top-level")
	assert.Equal("true", tr[2].Meta["_top_level"], "only 1 span for this service, should be top-level")
	assert.Equal("true", tr[3].Meta["_top_level"], "only 1 span for this service, should be top-level")
	assert.Nil(tr[4].Meta, "yet another sup span, not top-level")
}

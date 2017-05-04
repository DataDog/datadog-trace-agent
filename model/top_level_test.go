package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopLevelTypical(t *testing.T) {
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

func TestTopLevelSingle(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web"},
	}

	tr.ComputeTopLevel()

	assert.Equal("true", tr[0].Meta["_top_level"], "root span should be top-level")
}

func TestTopLevelEmpty(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{}

	tr.ComputeTopLevel()

	assert.Equal(0, len(tr), "trace should still be empty")
}

func TestTopLevelOneService(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "mcnulty", Type: "web"},
		Span{TraceID: 1, SpanID: 3, ParentID: 2, Service: "mcnulty", Type: "web"},
		Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web"},
		Span{TraceID: 1, SpanID: 4, ParentID: 1, Service: "mcnulty", Type: "web"},
		Span{TraceID: 1, SpanID: 5, ParentID: 1, Service: "mcnulty", Type: "web"},
	}

	tr.ComputeTopLevel()

	assert.Nil(tr[0].Meta, "just a sub-span, not top-level")
	assert.Nil(tr[1].Meta, "just a sub-span, not top-level")
	assert.Equal("true", tr[2].Meta["_top_level"], "root span should be top-level")
	assert.Nil(tr[3].Meta, "just a sub-span, not top-level")
	assert.Nil(tr[4].Meta, "just a sub-span, not top-level")
}

func TestTopLevelLocalRoot(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web"},
		Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "mcnulty", Type: "sql"},
		Span{TraceID: 1, SpanID: 3, ParentID: 2, Service: "master-db", Type: "sql"},
		Span{TraceID: 1, SpanID: 4, ParentID: 1, Service: "redis", Type: "redis"},
		Span{TraceID: 1, SpanID: 5, ParentID: 1, Service: "mcnulty", Type: ""},
		Span{TraceID: 1, SpanID: 6, ParentID: 4, Service: "redis", Type: "redis"},
		Span{TraceID: 1, SpanID: 7, ParentID: 4, Service: "redis", Type: "redis"},
	}

	tr.ComputeTopLevel()

	assert.Equal("true", tr[0].Meta["_top_level"], "root span should be top-level")
	assert.Nil(tr[1].Meta, "main service, and not a root span, not top-level")
	assert.Equal("true", tr[2].Meta["_top_level"], "only 1 span for this service, should be top-level")
	assert.Equal("true", tr[3].Meta["_top_level"], "top-level but not root")
	assert.Nil(tr[4].Meta, "yet another sup span, not top-level")
	assert.Nil(tr[5].Meta, "yet another sup span, not top-level")
	assert.Nil(tr[6].Meta, "yet another sup span, not top-level")
}

func TestTopLevelWithTag(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web", Meta: map[string]string{"env": "prod"}},
		Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "mcnulty", Type: "web", Meta: map[string]string{"env": "prod"}},
	}

	tr.ComputeTopLevel()

	assert.Equal("true", tr[0].Meta["_top_level"], "root span should be top-level")
	assert.Equal("prod", tr[0].Meta["env"], "env tag should still be here")
	assert.Equal("", tr[1].Meta["_top_level"], "not a top-level span")
	assert.Equal("prod", tr[1].Meta["env"], "env tag should still be here")
}

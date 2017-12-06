package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopLevelTypical(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		&Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web"},
		&Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "mcnulty", Type: "sql"},
		&Span{TraceID: 1, SpanID: 3, ParentID: 2, Service: "master-db", Type: "sql"},
		&Span{TraceID: 1, SpanID: 4, ParentID: 1, Service: "redis", Type: "redis"},
		&Span{TraceID: 1, SpanID: 5, ParentID: 1, Service: "mcnulty", Type: ""},
	}

	tr.ComputeTopLevel()

	assert.True(tr[0].TopLevel(), "root span should be top-level")
	assert.False(tr[1].TopLevel(), "main service, and not a root span, not top-level")
	assert.True(tr[2].TopLevel(), "only 1 span for this service, should be top-level")
	assert.True(tr[3].TopLevel(), "only 1 span for this service, should be top-level")
	assert.False(tr[4].TopLevel(), "yet another sup span, not top-level")
}

func TestTopLevelSingle(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		&Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web"},
	}

	tr.ComputeTopLevel()

	assert.True(tr[0].TopLevel(), "root span should be top-level")
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
		&Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "mcnulty", Type: "web"},
		&Span{TraceID: 1, SpanID: 3, ParentID: 2, Service: "mcnulty", Type: "web"},
		&Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web"},
		&Span{TraceID: 1, SpanID: 4, ParentID: 1, Service: "mcnulty", Type: "web"},
		&Span{TraceID: 1, SpanID: 5, ParentID: 1, Service: "mcnulty", Type: "web"},
	}

	tr.ComputeTopLevel()

	assert.False(tr[0].TopLevel(), "just a sub-span, not top-level")
	assert.False(tr[1].TopLevel(), "just a sub-span, not top-level")
	assert.True(tr[2].TopLevel(), "root span should be top-level")
	assert.False(tr[3].TopLevel(), "just a sub-span, not top-level")
	assert.False(tr[4].TopLevel(), "just a sub-span, not top-level")
}

func TestTopLevelLocalRoot(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		&Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web"},
		&Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "mcnulty", Type: "sql"},
		&Span{TraceID: 1, SpanID: 3, ParentID: 2, Service: "master-db", Type: "sql"},
		&Span{TraceID: 1, SpanID: 4, ParentID: 1, Service: "redis", Type: "redis"},
		&Span{TraceID: 1, SpanID: 5, ParentID: 1, Service: "mcnulty", Type: ""},
		&Span{TraceID: 1, SpanID: 6, ParentID: 4, Service: "redis", Type: "redis"},
		&Span{TraceID: 1, SpanID: 7, ParentID: 4, Service: "redis", Type: "redis"},
	}

	tr.ComputeTopLevel()

	assert.True(tr[0].TopLevel(), "root span should be top-level")
	assert.False(tr[1].TopLevel(), "main service, and not a root span, not top-level")
	assert.True(tr[2].TopLevel(), "only 1 span for this service, should be top-level")
	assert.True(tr[3].TopLevel(), "top-level but not root")
	assert.False(tr[4].TopLevel(), "yet another sup span, not top-level")
	assert.False(tr[5].TopLevel(), "yet another sup span, not top-level")
	assert.False(tr[6].TopLevel(), "yet another sup span, not top-level")
}

func TestTopLevelWithTag(t *testing.T) {
	assert := assert.New(t)

	tr := Trace{
		&Span{TraceID: 1, SpanID: 1, ParentID: 0, Service: "mcnulty", Type: "web", Metrics: map[string]float64{"custom": 42}},
		&Span{TraceID: 1, SpanID: 2, ParentID: 1, Service: "mcnulty", Type: "web", Metrics: map[string]float64{"custom": 42}},
	}

	tr.ComputeTopLevel()

	t.Logf("%v\n", tr[1].Metrics)

	assert.True(tr[0].TopLevel(), "root span should be top-level")
	assert.Equal(float64(42), tr[0].Metrics["custom"], "custom metric should still be here")
	assert.False(tr[1].TopLevel(), "not a top-level span")
	assert.Equal(float64(42), tr[1].Metrics["custom"], "custom metric should still be here")
}

func TestTopLevelGetSetBlackBox(t *testing.T) {
	assert := assert.New(t)

	span := Span{}

	assert.False(span.TopLevel(), "by default, all spans are considered non top-level")
	span.setTopLevel(true)
	assert.True(span.TopLevel(), "marked as top-level")
	span.setTopLevel(false)
	assert.False(span.TopLevel(), "no more top-level")

	span.Metrics = map[string]float64{"custom": 42}

	assert.False(span.TopLevel(), "by default, all spans are considered non top-level")
	span.setTopLevel(true)
	assert.True(span.TopLevel(), "marked as top-level")
	span.setTopLevel(false)
	assert.False(span.TopLevel(), "no more top-level")
}

func TestTopLevelGetSetMetrics(t *testing.T) {
	assert := assert.New(t)

	span := Span{}

	assert.Nil(span.Metrics, "no meta at all")
	span.setTopLevel(true)
	assert.Equal(float64(1), span.Metrics["_top_level"], "should have a _top_level:1 flag")
	span.setTopLevel(false)
	assert.Nil(span.Metrics, "no meta at all")

	span.Metrics = map[string]float64{"custom": 42}

	assert.False(span.TopLevel(), "still non top-level")
	span.setTopLevel(true)
	assert.Equal(float64(1), span.Metrics["_top_level"], "should have a _top_level:1 flag")
	assert.Equal(float64(42), span.Metrics["custom"], "former metrics should still be here")
	assert.True(span.TopLevel(), "marked as top-level")
	span.setTopLevel(false)
	assert.False(span.TopLevel(), "non top-level any more")
	assert.Equal(float64(0), span.Metrics["_top_level"], "should have no _top_level:1 flag")
	assert.Equal(float64(42), span.Metrics["custom"], "former metrics should still be here")
}

func TestForceMetrics(t *testing.T) {
	assert := assert.New(t)

	span := Span{}

	assert.False(span.ForceMetrics(), "by default, metrics are not enforced for sub name spans")
	span.Meta = map[string]string{"datadog.trace_metrics": "true"}
	assert.True(span.ForceMetrics(), "metrics should be enforced because tag is present")
	span.Meta = map[string]string{"env": "dev"}
	assert.False(span.ForceMetrics(), "there's a tag, but metrics should not be enforced anyway")
}

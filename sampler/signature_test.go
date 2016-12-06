package sampler

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestSignatureSimilar(t *testing.T) {
	assert := assert.New(t)
	t1 := model.Trace{
		model.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
		model.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		model.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		model.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := model.Trace{
		model.Span{TraceID: 102, SpanID: 1021, Service: "x1", Name: "y1", Resource: "z1", Duration: 992312},
		model.Span{TraceID: 102, SpanID: 1022, ParentID: 1021, Service: "x1", Name: "y1", Resource: "z1", Duration: 34347},
		model.Span{TraceID: 102, SpanID: 1023, ParentID: 1022, Service: "x2", Name: "y2", Resource: "z2", Duration: 349944},
	}

	assert.Equal(ComputeSignature(t1), ComputeSignature(t2))
}

func TestSignatureDifferentError(t *testing.T) {
	assert := assert.New(t)
	t1 := model.Trace{
		model.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
		model.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		model.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		model.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := model.Trace{
		model.Span{TraceID: 110, SpanID: 1101, Service: "x1", Name: "y1", Resource: "z1", Duration: 992312},
		model.Span{TraceID: 110, SpanID: 1102, ParentID: 1101, Service: "x1", Name: "y1", Resource: "z1", Error: 1, Duration: 34347},
		model.Span{TraceID: 110, SpanID: 1103, ParentID: 1101, Service: "x2", Name: "y2", Resource: "z2", Duration: 349944},
	}

	assert.NotEqual(ComputeSignature(t1), ComputeSignature(t2))
}

func TestSignatureDifferentRoot(t *testing.T) {
	assert := assert.New(t)
	t1 := model.Trace{
		model.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
		model.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		model.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		model.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := model.Trace{
		model.Span{TraceID: 103, SpanID: 1031, Service: "x1", Name: "y1", Resource: "z2", Duration: 19207},
		model.Span{TraceID: 103, SpanID: 1032, ParentID: 1031, Service: "x1", Name: "y1", Resource: "z1", Duration: 234923874},
		model.Span{TraceID: 103, SpanID: 1033, ParentID: 1032, Service: "x1", Name: "y1", Resource: "z1", Duration: 152342344},
	}

	assert.NotEqual(ComputeSignature(t1), ComputeSignature(t2))
}

package sampler

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/stretchr/testify/assert"
)

func testComputeSignature(trace agent.Trace) Signature {
	root := trace.GetRoot()
	env := trace.GetEnv()
	return computeSignatureWithRootAndEnv(trace, root, env)
}

func TestSignatureSimilar(t *testing.T) {
	assert := assert.New(t)

	t1 := agent.Trace{
		&agent.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
		&agent.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		&agent.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		&agent.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := agent.Trace{
		&agent.Span{TraceID: 102, SpanID: 1021, Service: "x1", Name: "y1", Resource: "z1", Duration: 992312},
		&agent.Span{TraceID: 102, SpanID: 1022, ParentID: 1021, Service: "x1", Name: "y1", Resource: "z1", Duration: 34347},
		&agent.Span{TraceID: 102, SpanID: 1023, ParentID: 1022, Service: "x2", Name: "y2", Resource: "z2", Duration: 349944},
	}

	assert.Equal(testComputeSignature(t1), testComputeSignature(t2))
}

func TestSignatureDifferentError(t *testing.T) {
	assert := assert.New(t)

	t1 := agent.Trace{
		&agent.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
		&agent.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		&agent.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		&agent.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := agent.Trace{
		&agent.Span{TraceID: 110, SpanID: 1101, Service: "x1", Name: "y1", Resource: "z1", Duration: 992312},
		&agent.Span{TraceID: 110, SpanID: 1102, ParentID: 1101, Service: "x1", Name: "y1", Resource: "z1", Error: 1, Duration: 34347},
		&agent.Span{TraceID: 110, SpanID: 1103, ParentID: 1101, Service: "x2", Name: "y2", Resource: "z2", Duration: 349944},
	}

	assert.NotEqual(testComputeSignature(t1), testComputeSignature(t2))
}

func TestSignatureDifferentRoot(t *testing.T) {
	assert := assert.New(t)

	t1 := agent.Trace{
		&agent.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
		&agent.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		&agent.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		&agent.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := agent.Trace{
		&agent.Span{TraceID: 103, SpanID: 1031, Service: "x1", Name: "y1", Resource: "z2", Duration: 19207},
		&agent.Span{TraceID: 103, SpanID: 1032, ParentID: 1031, Service: "x1", Name: "y1", Resource: "z1", Duration: 234923874},
		&agent.Span{TraceID: 103, SpanID: 1033, ParentID: 1032, Service: "x1", Name: "y1", Resource: "z1", Duration: 152342344},
	}

	assert.NotEqual(testComputeSignature(t1), testComputeSignature(t2))
}

func testComputeServiceSignature(trace agent.Trace) Signature {
	root := trace.GetRoot()
	env := trace.GetEnv()
	return ServiceSignature{root.Service, env}.Hash()
}

func TestServiceSignatureSimilar(t *testing.T) {
	assert := assert.New(t)

	t1 := agent.Trace{
		&agent.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
		&agent.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		&agent.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		&agent.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := agent.Trace{
		&agent.Span{TraceID: 102, SpanID: 1021, Service: "x1", Name: "y2", Resource: "z2", Duration: 992312},
		&agent.Span{TraceID: 102, SpanID: 1022, ParentID: 1021, Service: "x1", Name: "y1", Resource: "z1", Error: 1, Duration: 34347},
		&agent.Span{TraceID: 102, SpanID: 1023, ParentID: 1022, Service: "x2", Name: "y2", Resource: "z2", Duration: 349944},
	}
	assert.Equal(testComputeServiceSignature(t1), testComputeServiceSignature(t2))
}

func TestServiceSignatureDifferentService(t *testing.T) {
	assert := assert.New(t)

	t1 := agent.Trace{
		&agent.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
		&agent.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		&agent.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		&agent.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := agent.Trace{
		&agent.Span{TraceID: 103, SpanID: 1031, Service: "x2", Name: "y1", Resource: "z1", Duration: 19207},
		&agent.Span{TraceID: 103, SpanID: 1032, ParentID: 1031, Service: "x1", Name: "y1", Resource: "z1", Duration: 234923874},
		&agent.Span{TraceID: 103, SpanID: 1033, ParentID: 1032, Service: "x1", Name: "y1", Resource: "z1", Duration: 152342344},
	}

	assert.NotEqual(testComputeServiceSignature(t1), testComputeServiceSignature(t2))
}

func TestServiceSignatureDifferentEnv(t *testing.T) {
	assert := assert.New(t)

	t1 := agent.Trace{
		&agent.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965, Meta: map[string]string{"env": "test"}},
		&agent.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
		&agent.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
		&agent.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
	}
	t2 := agent.Trace{
		&agent.Span{TraceID: 110, SpanID: 1101, Service: "x1", Name: "y1", Resource: "z1", Duration: 992312, Meta: map[string]string{"env": "prod"}},
		&agent.Span{TraceID: 110, SpanID: 1102, ParentID: 1101, Service: "x1", Name: "y1", Resource: "z1", Duration: 34347},
		&agent.Span{TraceID: 110, SpanID: 1103, ParentID: 1101, Service: "x2", Name: "y2", Resource: "z2", Duration: 349944},
	}

	assert.NotEqual(testComputeServiceSignature(t1), testComputeServiceSignature(t2))
}

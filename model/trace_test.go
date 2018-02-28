package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRootFromCompleteTrace(t *testing.T) {
	assert := assert.New(t)

	trace := Trace{
		&Span{TraceID: uint64(1234), SpanID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
		&Span{TraceID: uint64(1234), SpanID: uint64(12342), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
		&Span{TraceID: uint64(1234), SpanID: uint64(12343), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
		&Span{TraceID: uint64(1234), SpanID: uint64(12344), ParentID: uint64(12342), Service: "s2", Name: "n2", Resource: ""},
		&Span{TraceID: uint64(1234), SpanID: uint64(12345), ParentID: uint64(12344), Service: "s2", Name: "n2", Resource: ""},
	}

	assert.Equal(trace.GetRoot().SpanID, uint64(12341))
}

func TestGetRootFromPartialTrace(t *testing.T) {
	assert := assert.New(t)

	trace := Trace{
		&Span{TraceID: uint64(1234), SpanID: uint64(12341), ParentID: uint64(12340), Service: "s1", Name: "n1", Resource: ""},
		&Span{TraceID: uint64(1234), SpanID: uint64(12342), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
		&Span{TraceID: uint64(1234), SpanID: uint64(12343), ParentID: uint64(12342), Service: "s2", Name: "n2", Resource: ""},
	}

	assert.Equal(trace.GetRoot().SpanID, uint64(12341))
}

func TestTraceChildrenMap(t *testing.T) {
	assert := assert.New(t)

	trace := Trace{
		&Span{SpanID: 1, ParentID: 0},
		&Span{SpanID: 2, ParentID: 1},
		&Span{SpanID: 3, ParentID: 1},
		&Span{SpanID: 4, ParentID: 2},
		&Span{SpanID: 5, ParentID: 3},
		&Span{SpanID: 6, ParentID: 4},
	}

	childrenMap := trace.ChildrenMap()

	assert.Equal([]*Span{trace[1], trace[2]}, childrenMap[1])
	assert.Equal([]*Span{trace[3]}, childrenMap[2])
	assert.Equal([]*Span{trace[4]}, childrenMap[3])
	assert.Equal([]*Span{trace[5]}, childrenMap[4])
	assert.Equal([]*Span{}, childrenMap[5])
	assert.Equal([]*Span{}, childrenMap[6])
}

func TestExtractTopLevelSubtracesWithSimpleTrace(t *testing.T) {
	assert := assert.New(t)

	trace := Trace{
		&Span{SpanID: 1, ParentID: 0, Service: "s1"},
		&Span{SpanID: 2, ParentID: 1, Service: "s2"},
		&Span{SpanID: 3, ParentID: 2, Service: "s2"},
		&Span{SpanID: 4, ParentID: 3, Service: "s2"},
		&Span{SpanID: 5, ParentID: 1, Service: "s1"},
	}

	expected := []Subtrace{
		Subtrace{trace[0], trace},
		Subtrace{trace[1], []*Span{trace[1], trace[2], trace[3]}},
	}

	trace.ComputeTopLevel()
	subtraces := trace.ExtractTopLevelSubtraces(trace[0])

	assert.Equal(len(expected), len(subtraces))

	subtracesMap := make(map[*Span]Subtrace)
	for _, s := range subtraces {
		subtracesMap[s.Root] = s
	}

	for _, s := range expected {
		assert.ElementsMatch(s.Trace, subtracesMap[s.Root].Trace)
	}
}

func TestExtractTopLevelSubtracesShouldIgnoreLeafTopLevel(t *testing.T) {
	assert := assert.New(t)

	trace := Trace{
		&Span{SpanID: 1, ParentID: 0, Service: "s1"},
		&Span{SpanID: 2, ParentID: 1, Service: "s2"},
		&Span{SpanID: 3, ParentID: 2, Service: "s2"},
		&Span{SpanID: 4, ParentID: 1, Service: "s3"},
	}

	expected := []Subtrace{
		Subtrace{trace[0], trace},
		Subtrace{trace[1], []*Span{trace[1], trace[2]}},
	}

	trace.ComputeTopLevel()
	subtraces := trace.ExtractTopLevelSubtraces(trace[0])

	assert.Equal(len(expected), len(subtraces))

	subtracesMap := make(map[*Span]Subtrace)
	for _, s := range subtraces {
		subtracesMap[s.Root] = s
	}

	for _, s := range expected {
		assert.ElementsMatch(s.Trace, subtracesMap[s.Root].Trace)
	}
}

func TestExtractTopLevelSubtracesWorksInSpiteOfCycles(t *testing.T) {
	assert := assert.New(t)

	trace := Trace{
		&Span{SpanID: 1, ParentID: 3, Service: "s1"},
		&Span{SpanID: 2, ParentID: 1, Service: "s2"},
		&Span{SpanID: 3, ParentID: 2, Service: "s2"},
	}

	expected := []Subtrace{
		Subtrace{trace[0], trace},
		Subtrace{trace[1], []*Span{trace[1], trace[2]}},
	}

	trace.ComputeTopLevel()
	subtraces := trace.ExtractTopLevelSubtraces(trace[0])

	assert.Equal(len(expected), len(subtraces))

	subtracesMap := make(map[*Span]Subtrace)
	for _, s := range subtraces {
		subtracesMap[s.Root] = s
	}

	for _, s := range expected {
		assert.ElementsMatch(s.Trace, subtracesMap[s.Root].Trace)
	}
}

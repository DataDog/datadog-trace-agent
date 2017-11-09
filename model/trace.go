package model

import (
	pb "github.com/DataDog/datadog-trace-agent/model/protobuf"
	log "github.com/cihub/seelog"
)

//go:generate msgp -marshal=false

// Trace is a collection of spans with the same trace ID
type Trace []Span

// Traces is a list of traces that represents the  ...
type Traces []Trace

// GetEnv returns the meta value for the "env" key for
// the first trace it finds or an empty string
func (t Trace) GetEnv() string {
	// exit this on first success
	for _, s := range t {
		for k, v := range s.Meta {
			if k == "env" {
				return v
			}
		}
	}
	return ""
}

// GetRoot extracts the root span from a trace
func (t Trace) GetRoot() *Span {
	// That should be caught beforehand
	if len(t) == 0 {
		return nil
	}
	// General case: go over all spans and check for one which matching parent
	parentIDToChild := map[uint64]*Span{}

	for i := range t {
		// Common case optimization: check for span with ParentID == 0, starting from the end,
		// since some clients report the root last
		j := len(t) - 1 - i
		if t[j].ParentID == 0 {
			return &t[j]
		}
		parentIDToChild[t[j].ParentID] = &t[j]
	}

	for i := range t {
		if _, ok := parentIDToChild[t[i].SpanID]; ok {
			delete(parentIDToChild, t[i].SpanID)
		}
	}

	// Here, if the trace is valid, we should have len(parentIDToChild) == 1
	if len(parentIDToChild) != 1 {
		log.Debugf("didn't reliably find the root span for traceID:%v", t[0].TraceID)
	}

	// Have a safe bahavior if that's not the case
	// Pick the first span without its parent
	for parentID := range parentIDToChild {
		return parentIDToChild[parentID]
	}

	// Gracefully fail with the last span of the trace
	return &t[len(t)-1]
}

// ChildrenMap returns a map containing for each span id the list of its
// direct children.
func (t Trace) ChildrenMap() map[uint64]Spans {
	childrenMap := make(map[uint64]Spans)

	for i := range t {
		span := &t[i]

		if span.ParentID == 0 {
			continue
		}

		children, ok := childrenMap[span.SpanID]
		if !ok {
			childrenMap[span.SpanID] = Spans{}
		}

		children, ok = childrenMap[span.ParentID]
		if ok {
			children = append(children, span)
		} else {
			children = Spans{span}
		}
		childrenMap[span.ParentID] = children
	}

	return childrenMap
}

// NewTraceFlushMarker returns a trace with a single span as flush marker
func NewTraceFlushMarker() Trace {
	return []Span{NewFlushMarker()}
}

// ComputeWeight sets the weight private attribute to the weight
// of the root Span. This is because sampling ratio is stored as a metrics
// only is the root span, but is required for computing values in any span.
func (t Trace) ComputeWeight(root Span) {
	weight := root.Weight()

	for i := range t {
		t[i].weight = weight
	}
}

func (t Trace) ToProto() *pb.Trace {
	root := t[0]

	return &pb.Trace{
		TraceID: root.TraceID,
		Spans: []*pb.Span{
			root.ToProto(),
		},
		StartTime: root.Start,
		EndTime:   root.Start + root.Duration,
	}
}

func ProtoToTrace(t *pb.Trace) Trace {
	spans := make([]Span, 0, len(t.Spans))
	for _, s := range t.Spans {
		spans = append(spans, ProtoToSpan(s))
	}

	return Trace(spans)
}

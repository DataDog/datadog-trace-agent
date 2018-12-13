package agent

import (
	log "github.com/cihub/seelog"
)

//go:generate sh -c "protoc -I=. -I=../vendor --gogofaster_out=. *.proto"
//go:generate msgp -file=span.pb.go -marshal=false
//go:generate msgp -marshal=false

// Trace is a collection of spans with the same trace ID
type Trace []*Span

// Traces is a list of traces. This model matters as this is what we unpack from msgp.
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
			return t[j]
		}
		parentIDToChild[t[j].ParentID] = t[j]
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
	return t[len(t)-1]
}

// ChildrenMap returns a map containing for each span id the list of its
// direct children.
func (t Trace) ChildrenMap() map[uint64][]*Span {
	childrenMap := make(map[uint64][]*Span)

	for i := range t {
		span := t[i]

		if span.ParentID == 0 {
			continue
		}

		children, ok := childrenMap[span.SpanID]
		if !ok {
			childrenMap[span.SpanID] = []*Span{}
		}

		children, ok = childrenMap[span.ParentID]
		if ok {
			children = append(children, span)
		} else {
			children = []*Span{span}
		}
		childrenMap[span.ParentID] = children
	}

	return childrenMap
}

// NewTraceFlushMarker returns a trace with a single span as flush marker
func NewTraceFlushMarker() Trace {
	return []*Span{NewFlushMarker()}
}

// APITrace returns an APITrace from the trace, as required by the Datadog API.
func (t Trace) APITrace() *APITrace {
	start := t[0].Start
	end := t[0].End()
	for i := range t {
		if t[i].Start < start {
			start = t[i].Start
		}
		if t[i].End() < end {
			end = t[i].End()
		}
	}

	return &APITrace{
		TraceID:   t[0].TraceID,
		Spans:     t,
		StartTime: start,
		EndTime:   end,
	}
}

// Subtrace represents the combination of a root span and the trace consisting of all its descendant spans
type Subtrace struct {
	Root  *Span
	Trace Trace
}

// spanAndAncestors is used by ExtractTopLevelSubtraces to store the pair of a span and its ancestors
type spanAndAncestors struct {
	Span      *Span
	Ancestors []*Span
}

// element and queue implement a very basic LIFO used to do an iterative DFS on a trace
type element struct {
	SpanAndAncestors *spanAndAncestors
	Next             *element
}

type stack struct {
	head *element
}

func (s *stack) Push(value *spanAndAncestors) {
	e := &element{value, nil}
	if s.head == nil {
		s.head = e
		return
	}
	e.Next = s.head
	s.head = e
}

func (s *stack) Pop() *spanAndAncestors {
	if s.head == nil {
		return nil
	}
	value := s.head.SpanAndAncestors
	s.head = s.head.Next
	return value
}

// ExtractTopLevelSubtraces extracts all subtraces rooted in a toplevel span,
// ComputeTopLevel should be called before.
func (t Trace) ExtractTopLevelSubtraces(root *Span) []Subtrace {
	if root == nil {
		return []Subtrace{}
	}
	childrenMap := t.ChildrenMap()
	subtraces := []Subtrace{}

	visited := make(map[*Span]bool, len(t))
	subtracesMap := make(map[*Span][]*Span)
	var next stack
	next.Push(&spanAndAncestors{root, []*Span{}})

	// We do a DFS on the trace to record the toplevel ancesters of each span
	for current := next.Pop(); current != nil; current = next.Pop() {
		// We do not extract subtraces for toplevel spans that have no children
		// since these are not interresting
		if current.Span.TopLevel() && len(childrenMap[current.Span.SpanID]) > 0 {
			current.Ancestors = append(current.Ancestors, current.Span)
		}
		visited[current.Span] = true
		for _, ancestor := range current.Ancestors {
			subtracesMap[ancestor] = append(subtracesMap[ancestor], current.Span)
		}
		for _, child := range childrenMap[current.Span.SpanID] {
			// Continue if this span has already been explored (meaning the
			// trace is not a Tree)
			if visited[child] {
				log.Warnf("Found a cycle while processing traceID:%v, trace should be a tree", t[0].TraceID)
				continue
			}
			next.Push(&spanAndAncestors{child, current.Ancestors})
		}
	}

	for topLevel, subtrace := range subtracesMap {
		subtraces = append(subtraces, Subtrace{topLevel, subtrace})
	}

	return subtraces
}

// Clone does a deep copy of the Trace
func (t Trace) Clone() Trace {
	clonedTrace := make([]*Span, len(t))
	for i := range t {
		clonedSpan := *t[i]
		clonedSpan.Meta = make(map[string]string, len(t[i].Meta))
		for k, v := range t[i].Meta {
			clonedSpan.Meta[k] = v
		}
		clonedSpan.Metrics = make(map[string]float64, len(t[i].Metrics))
		for k, v := range t[i].Metrics {
			clonedSpan.Metrics[k] = v
		}
		clonedTrace[i] = &clonedSpan
	}
	return clonedTrace
}

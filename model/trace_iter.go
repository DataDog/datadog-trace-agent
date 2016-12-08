package model

import "errors"

var (
	// ErrEOT indicates we reached the end of a trace (e.g. for an iterator)
	ErrEOT = errors.New("end of trace")
	// ErrEOL indicates we reached the end of a trace level
	ErrEOL = errors.New("end of level")
)

// TraceLevelIterator iterates through a trace by returning spans of increasing depth levels
type TraceLevelIterator struct {
	parents    map[uint64]struct{}
	visited    map[uint64]struct{}
	idxvisited map[int]struct{}
	cursor     int

	trace Trace
}

// NewTraceLevelIterator returns a new iterator on the given trace
func NewTraceLevelIterator(t Trace) *TraceLevelIterator {
	// TODO[leo]: potentially reduce allocs
	return &TraceLevelIterator{
		parents:    map[uint64]struct{}{0: struct{}{}}, // for the root
		visited:    make(map[uint64]struct{}),
		idxvisited: make(map[int]struct{}),
		trace:      t,
	}
}

// NextSpan returns the next span at this level or ErrEOL
func (tl *TraceLevelIterator) NextSpan() (*Span, error) {
	var ok bool
	for tl.cursor < len(tl.trace) {
		// already visited, next
		if _, ok = tl.idxvisited[tl.cursor]; ok {
			tl.cursor++
			continue
		}

		// if that span's parent is not acceptable for this level, next
		if _, ok = tl.parents[tl.trace[tl.cursor].ParentID]; !ok {
			tl.cursor++
			continue
		}

		// mark that span as visited and return it
		tl.idxvisited[tl.cursor] = struct{}{}
		tl.visited[tl.trace[tl.cursor].SpanID] = struct{}{}
		tl.cursor++
		return &tl.trace[tl.cursor-1], nil
	}

	// nothing is available at this level
	return nil, ErrEOL
}

// NextLevel goes one level deeper, or returns ErrEOT if no more are available
func (tl *TraceLevelIterator) NextLevel() error {
	if len(tl.idxvisited) == len(tl.trace) {
		return ErrEOT
	}
	if len(tl.parents) == 0 {
		return ErrEOT
	}

	tl.cursor = 0
	tl.parents = tl.visited
	tl.visited = make(map[uint64]struct{})

	return nil
}

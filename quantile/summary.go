package quantile

import (
	"math/rand"
)

/*
"Space-Efficient Online Computation of Quantile Summaries" (Greenwald, Khanna 2001)

http://infolab.stanford.edu/~datar/courses/cs361a/papers/quantiles.pdf

This implementation is backed by a skiplist to make inserting elements into the
summary faster.  Querying is still O(n).

*/

// EPSILON is the precision of the rank returned by our quantile queries
const EPSILON float64 = 0.01

// Entry is an element of the skiplist, see GK paper for description
type Entry struct {
	V     float64 `json:"v"`
	G     int     `json:"g"`
	Delta int     `json:"delta"`
}

// SummarySlice reprensents how many values are in a [Start, End] range
type SummarySlice struct {
	Start  float64
	End    float64
	Weight int
}

const maxHeight = 31

// Skiplist is a pseudo-random data structure used to store nodes and find quickly what we want
type Skiplist struct {
	height int
	head   *SkiplistNode
}

// SkiplistNode is holding the actual value and pointers to the neighbor nodes
type SkiplistNode struct {
	value Entry
	next  []*SkiplistNode
	prev  []*SkiplistNode
}

// NewSkiplist returns a new empty Skiplist
func NewSkiplist() *Skiplist {
	return &Skiplist{
		height: 0,
		head:   &SkiplistNode{next: make([]*SkiplistNode, maxHeight)},
	}
}

// Insert adds a new Entry to the Skiplist and yields a pointer to the node where the data was inserted
func (s *Skiplist) Insert(e Entry) *SkiplistNode {
	level := 0

	n := rand.Int31()
	for n&1 == 1 {
		level++
		n >>= 1
	}

	if level > s.height {
		s.height++
		level = s.height
	}

	node := &SkiplistNode{
		value: e,
		next:  make([]*SkiplistNode, level+1),
		prev:  make([]*SkiplistNode, level+1),
	}
	curr := s.head
	for i := s.height; i >= 0; i-- {

		for curr.next[i] != nil && e.V >= curr.next[i].value.V {
			curr = curr.next[i]
		}

		if i > level {
			continue
		}

		node.next[i] = curr.next[i]
		if curr.next[i] != nil && curr.next[i].prev[i] != nil {
			curr.next[i].prev[i] = node
		}
		curr.next[i] = node
		node.prev[i] = curr
	}

	return node
}

// Remove removes a node from the Skiplist
func (s *Skiplist) Remove(node *SkiplistNode) {

	// remove n from each level of the Skiplist

	for i := range node.next {
		prev := node.prev[i]
		next := node.next[i]

		if prev != nil {
			prev.next[i] = next
		}
		if next != nil {
			next.prev[i] = prev
		}
		node.next[i] = nil
		node.prev[i] = nil
	}
}

package quantile

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unsafe"
)

/*
FIXME: shamelessly copied from dgryski/go-gk, not verified, not really tested
Should reimplement everything from scratch from the paper

"Space-Efficient Online Computation of Quantile Summaries" (Greenwald, Khanna 2001)

http://infolab.stanford.edu/~datar/courses/cs361a/papers/quantiles.pdf

This implementation is backed by a skiplist to make inserting elements into the
summary faster.  Querying is still O(n).

*/

const epsilon float64 = 0.01

type Summary struct {
	data        *Skiplist
	EncodedData []Entry `json:"data"`
	N           int     `json:"n"`
}

// Entry is an element of the skiplist
type Entry struct {
	V       int64    `json:"v"`
	G       int      `json:"g"`
	Delta   int      `json:"delta"`
	Samples []uint64 `json:"samples"`
}

// NewSummary returns a new approx-summary with accuracy epsilon (0 <= epsilon <= 1)
func NewSummary() *Summary {
	return &Summary{
		data: NewSkiplist(),
	}
}

// Encode prepares a flat version of the skiplist for various encoders (json/gob)
func (s Summary) MarshalJSON() ([]byte, error) {
	if s.data == nil {
		panic(errors.New("Cannot marshal non-initialized Summary"))
	}

	// TODO[leo] preallocate, not sure: 1/ 2*epsilon?
	s.EncodedData = make([]Entry, 0)
	curr := s.data.head
	for curr != nil {
		s.EncodedData = append(s.EncodedData, curr.value)
		curr = curr.next[0]
	}

	return json.Marshal(map[string]interface{}{
		"data": s.EncodedData,
		"n":    s.N,
	})
}

type summary Summary

// Decode is used to restore the original skiplist from the EncodedData
func (s *Summary) UnmarshalJSON(b []byte) error {
	ss := summary{}
	err := json.Unmarshal(b, &ss)
	if err != nil {
		return err
	}
	*s = Summary(ss)

	s.data = NewSkiplist()
	for _, e := range s.EncodedData {
		s.data.Insert(e)
	}

	return nil
}

// Insert inserts an item into the quantile summary
func (s *Summary) Insert(v int64, t uint64) {
	e := Entry{
		V:       v,
		G:       1,
		Delta:   0,
		Samples: []uint64{t},
	}

	eptr := s.data.Insert(e)

	s.N++

	if eptr.prev[0] != s.data.head && eptr.next[0] != nil {
		eptr.value.Delta = int(2 * epsilon * float64(s.N))
	}

	if s.N%int(1.0/float64(2.0*epsilon)) == 0 {
		s.compress()
	}
}

func (s *Summary) compress() {
	var missing int

	epsN := int(2 * epsilon * float64(s.N))

	for elt := s.data.head.next[0]; elt != nil && elt.next[0] != nil; {
		next := elt.next[0]
		t := elt.value
		nt := &next.value

		// value merging
		if t.V == nt.V {
			missing += nt.G
			nt.Delta += missing
			nt.G = t.G
			nt.Samples = append(nt.Samples, t.Samples...)
			s.data.Remove(elt)
		} else if t.G+nt.G+missing+nt.Delta < epsN {
			nt.G += t.G + missing
			nt.Samples = append(nt.Samples, t.Samples...)
			missing = 0
			s.data.Remove(elt)
		} else {
			nt.G += missing
			missing = 0
		}
		elt = next
	}
}

// Quantile returns an epsilon estimate of the element at quantile 'q' (0 <= q <= 1)
func (s *Summary) Quantile(q float64) (int64, []uint64) {

	// convert quantile to rank
	r := int(q*float64(s.N) + 0.5)

	var rmin int
	epsN := int(epsilon * float64(s.N))

	for elt := s.data.head.next[0]; elt != nil; elt = elt.next[0] {
		t := elt.value
		rmin += t.G
		n := elt.next[0]

		if n == nil {
			return t.V, t.Samples
		}

		if r+epsN < rmin+n.value.G+n.value.Delta {
			if r+epsN < rmin+n.value.G {
				return t.V, t.Samples
			}
			return n.value.V, n.value.Samples
		}
	}

	panic("not reached")
}

const maxHeight = 31

// Skiplist is a? (TODO LEO)
type Skiplist struct {
	height int
	head   *SkiplistNode
	rnd    *rand.Rand
}

// SkiplistNode is a? (TODO LEO)
type SkiplistNode struct {
	value Entry
	next  []*SkiplistNode
	prev  []*SkiplistNode
}

// Println prints deb8gug stuff? (TODO LEO)
func (n *SkiplistNode) Println(offset int, alreadySeen *map[uintptr]bool) {
	if _, ok := (*alreadySeen)[uintptr(unsafe.Pointer(n))]; ok {
		return
	}
	stroff := strings.Repeat(" ", offset)
	fmt.Printf("%sENTRY {v: %d, g: %d, delta:%d, tids: %v}\n", stroff, n.value.V, n.value.G, n.value.Delta, n.value.Samples)
	fmt.Printf("%sPTR %p\n", stroff, n)
	fmt.Printf("%sNEXT:\n", stroff)
	(*alreadySeen)[uintptr(unsafe.Pointer(n))] = true

	for _, nptr := range n.next {
		if nptr != nil {
			nptr.Println(offset+1, alreadySeen)
		}
	}

	fmt.Printf("%sPREV:\n", stroff)

	for _, nptr := range n.prev {
		if nptr != nil {
			nptr.Println(offset+1, alreadySeen)
		}
	}
}

// NewSkiplist returns a new empty Skiplist
func NewSkiplist() *Skiplist {
	return &Skiplist{
		height: 0,
		head:   &SkiplistNode{next: make([]*SkiplistNode, maxHeight)},
		rnd:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Insert adds a SkiplistNode into a Skiplist while doing stuff? (TODO LEO)
func (s *Skiplist) Insert(e Entry) *SkiplistNode {
	level := 0

	n := s.rnd.Int31()
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

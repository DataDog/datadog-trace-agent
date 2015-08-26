package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
	"unsafe"
)

// Summary represents
type Summary interface {
	// Insert takes a value and a trace ID to insert it in our stats
	Insert(int64, uint64)
	// Quantile takes a q/n-style float quantile query and returns the corresponding value in our distribution and IDs of representative traces
	Quantile(float64) (int64, []uint64)
}

// NewSummary generates a new summary that answers to quantiles query with epsilon precision
func NewSummary(epsilon float64) Summary {
	if epsilon == 0 {
		return NewExactSummary()
	}

	return NewGKSummary(epsilon)
}

// FIXME: shamelessly copied from dgryski/go-gk, not verified, not really tested
// Should reimplement everything from scratch from the paper

// ExactSummary is an exact (map-based) quantile summary
// WARNING: IT IS NOT MADE TO BE USED ON PRODUCTION, IT WILL PROBABLY BLOW UP YOUR MEMORY
// BECAUSE WE KEEP EVERY SINGLE VALUE.
// IT IS MADE TO BE A "TRUE" REFERENCE FOR QUANTILES, USEFUL TO TEST NEW APPROXIMATION ALGORITHMS
// TO ACHIEVE WHAT WE WANT WITH THE LEAST RESOURCES POSSIBLE.
// NOTE: []uint64 is used in place of a []float because it's an optimized map type supposed to be 100x faster (</quote>), see float_slice.go:FloatBitsSlice
// there is a benchmark for that, but it's not really good ^
type ExactSummary struct {
	data    map[uint64]int      // counts of values, values represented on 64bits IEEE 754 rep
	samples map[uint64][]uint64 // for each datum, sample trace IDs
	n       int                 // stream length
	sorted  FloatBitsSlice      // cached sorted version of summary keys
}

// NewExactSummary returns a new empty exact distribution
func NewExactSummary() *ExactSummary {
	return &ExactSummary{
		data:    make(map[uint64]int),
		samples: make(map[uint64][]uint64),
	}
}

// Insert adds a value/trace ID tuple in its data structs
func (s *ExactSummary) Insert(v int64, t uint64) {
	s.n++

	uv := uint64(v)
	s.data[uv]++

	// clear out the cache of sorted keys
	s.sorted = nil

	if _, ok := s.samples[uv]; !ok {
		s.samples[uv] = []uint64{}
	}

	s.samples[uv] = append(s.samples[uv], t)
}

// Quantile returns the quantile representing a specific value
func (s *ExactSummary) Quantile(q float64) (int64, []uint64) {
	// re-create the cache
	if s.sorted == nil {
		s.sorted = make([]uint64, 0, len(s.data))

		for k := range s.data {
			s.sorted = append(s.sorted, k)
		}

		sort.Sort(s.sorted)
	}

	// TODO(dgryski): create prefix sum array and then binsearch to find quantile.
	total := 0

	for _, k := range s.sorted {
		total += s.data[k]
		p := float64(total) / float64(s.n)
		if q <= p {
			return int64(k), s.samples[k]
		}
	}

	panic("ExactSummary.Quantile(), end reached")
}

/*

"Space-Efficient Online Computation of Quantile Summaries" (Greenwald, Khanna 2001)

http://infolab.stanford.edu/~datar/courses/cs361a/papers/quantiles.pdf

This implementation is backed by a skiplist to make inserting elements into the
summary faster.  Querying is still O(n).

*/

// GKSummary see above
type GKSummary struct {
	data        *GKSkiplist
	EncodedData []GKEntry `json:"data"`
	Epsilon     float64   `json:"epsilon"`
	N           int       `json:"n"`
}

// GKEntry is an element of the skiplist
type GKEntry struct {
	V       int64    `json:"v"`
	G       int      `json:"g"`
	Delta   int      `json:"delta"`
	Samples []uint64 `json:"samples"`
}

// NewGKSummary returns a new approx-summary with accuracy epsilon (0 <= epsilon <= 1)
func NewGKSummary(epsilon float64) *GKSummary {
	return &GKSummary{
		Epsilon: epsilon,
		data:    NewGKSkiplist(),
	}
}

// Encode prepares a flat version of the skiplist for various encoders (json/gob)
func (s *GKSummary) Encode() {
	if s.data == nil {
		panic(errors.New("Cannot encode non-initialized Summary"))
	}

	// TODO[leo] preallocate, not sure: 1/ 2*epsilon?
	s.EncodedData = make([]GKEntry, 0)
	curr := s.data.head
	for curr != nil {
		s.EncodedData = append(s.EncodedData, curr.value)
		curr = curr.next[0]
	}
}

// Decode is used to restore the original skiplist from the EncodedData
func (s *GKSummary) Decode() {
	if s.EncodedData == nil {
		panic(errors.New("Nothing to decode"))
	}
	if s.data != nil {
		panic(errors.New("Cannot decode in already used struct"))
	}

	for _, e := range s.EncodedData {
		s.data.Insert(e)
	}
}

// Insert inserts an item into the quantile summary
func (s *GKSummary) Insert(v int64, t uint64) {
	e := GKEntry{
		V:       v,
		G:       1,
		Delta:   0,
		Samples: []uint64{t},
	}

	eptr := s.data.Insert(e)

	s.N++

	if eptr.prev[0] != s.data.head && eptr.next[0] != nil {
		eptr.value.Delta = int(2 * s.Epsilon * float64(s.N))
	}

	if s.N%int(1.0/float64(2.0*s.Epsilon)) == 0 {
		s.compress()
	}
}

func (s *GKSummary) compress() {
	var missing int

	epsN := int(2 * s.Epsilon * float64(s.N))

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
func (s *GKSummary) Quantile(q float64) (int64, []uint64) {

	// convert quantile to rank
	r := int(q*float64(s.N) + 0.5)

	var rmin int
	epsN := int(s.Epsilon * float64(s.N))

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

// GKSkiplist is a? (TODO LEO)
type GKSkiplist struct {
	height int
	head   *GKSkiplistNode
	rnd    *rand.Rand
}

// GKSkiplistNode is a? (TODO LEO)
type GKSkiplistNode struct {
	value GKEntry
	next  []*GKSkiplistNode
	prev  []*GKSkiplistNode
}

// Println prints deb8gug stuff? (TODO LEO)
func (n *GKSkiplistNode) Println(offset int, alreadySeen *map[uintptr]bool) {
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

// NewGKSkiplist returns a new empty GKSkiplist
func NewGKSkiplist() *GKSkiplist {
	return &GKSkiplist{
		height: 0,
		head:   &GKSkiplistNode{next: make([]*GKSkiplistNode, maxHeight)},
		rnd:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Insert adds a GKSkiplistNode into a GKSkiplist while doing stuff? (TODO LEO)
func (s *GKSkiplist) Insert(e GKEntry) *GKSkiplistNode {
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

	node := &GKSkiplistNode{
		value: e,
		next:  make([]*GKSkiplistNode, level+1),
		prev:  make([]*GKSkiplistNode, level+1),
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

// Remove removes a node from the GKSkiplist
func (s *GKSkiplist) Remove(node *GKSkiplistNode) {

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

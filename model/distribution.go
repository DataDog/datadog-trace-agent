package model

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
	"unsafe"
)

// Distribution is the common interface to account for a distribution of values and representative traces of each slice of values
type Distribution interface {
	// Insert takes a value and a trace ID to insert it in our stats and returns a boolean to indicate if this trace should be kept (ie. NOT sampled out)
	Insert(float64, SID) bool
	// Quantile takes a q/n-style float quantile query and returns the corresponding value in our distribution and IDs of representative traces
	Quantile(float64) (float64, []SID)
}

// NewDistribution generates a new distribution with a given epsilon
func NewDistribution(eps float64) Distribution {
	if eps == 0 {
		return NewExactDistro()
	}

	return NewGKDistro(eps)
}

// FIXME: shamelessly copied from dgryski/go-gk, not verified, not really tested
// Should reimplement everything from scratch from the paper

// ExactDistro is an exact (map-based) quantile summary
// WARNING!! IT IS NOT MADE TO BE USED ON PRODUCTION, IT WILL PROBABLY BLOW UP YOUR MEMORY
// BECAUSE WE KEEP EVERY SINGLE VALUE.
// IT IS MADE TO BE A "TRUE" REFERENCE FOR QUANTILES, USEFUL TO TEST NEW APPROXIMATION ALGORITHMS
// TO ACHIEVE WHAT WE WANT WITH THE LEAST RESOURCES POSSIBLE.
// []uint64 is used in place of a []float because it's an optimized map type supposed to be 100x faster (</quote>), see float_slice.go:FloatBitsSlice
type ExactDistro struct {
	summary map[uint64]int // counts of values, values represented on 64bits IEEE 754 rep
	samples map[uint64]SID // a trace ID that represents a
	n       int            // stream length
	keys    FloatBitsSlice // cached sorted version of summary keys
}

// NewExactDistro returns a new empty exact distribution
func NewExactDistro() *ExactDistro {
	return &ExactDistro{
		summary: make(map[uint64]int),
		samples: make(map[uint64]SID),
	}
}

// Insert adds a span to an exact distribution
func (d *ExactDistro) Insert(v float64, t SID) bool {
	d.n++

	vbits := math.Float64bits(v)
	d.summary[vbits]++

	// clear out the cache of sorted keys
	d.keys = nil

	if _, ok := d.samples[vbits]; ok {
		// FIXME, dumb but ?
		// Already here, so decide to drop this trace
		return false
	}

	d.samples[vbits] = t
	return true
}

// Quantile returns the quantile representing a specific value
func (d *ExactDistro) Quantile(q float64) (float64, []SID) {
	// re-create the cache
	if d.keys == nil {
		d.keys = make([]uint64, 0, len(d.summary))

		for k := range d.summary {
			d.keys = append(d.keys, k)
		}

		sort.Sort(d.keys)
	}

	// TODO(dgryski): create prefix sum array and then binsearch to find quantile.
	total := 0

	for _, k := range d.keys {
		total += d.summary[k]
		p := float64(total) / float64(d.n)
		if q <= p {
			return math.Float64frombits(k), []SID{d.samples[k]}
		}
	}

	panic("ExactDistro.Quantile(), end reached")
}

/*

"Space-Efficient Online Computation of Quantile Summaries" (Greenwald, Khanna 2001)

http://infolab.stanford.edu/~datar/courses/cs361a/papers/quantiles.pdf

This implementation is backed by a skiplist to make inserting elements into the
summary faster.  Querying is still O(n).

*/

// GKDistro is a quantile summary
type GKDistro struct {
	summary *GKSkiplist
	eps     float64
	n       int
}

// GKEntry is an element of a GKDistro
type GKEntry struct {
	V       float64 `json:"v"`
	G       int     `json:"g"`
	Delta   int     `json:"delta"`
	Samples []SID   `json:"samples"`
}

// NewGKDistro returns a new stream with accuracy epsilon (0 <= epsilon <= 1)
func NewGKDistro(eps float64) *GKDistro {
	return &GKDistro{
		eps:     eps,
		summary: NewGKSkiplist(),
	}
}

// MarshalJSON returns a JSON representation
func (s *GKDistro) MarshalJSON() ([]byte, error) {
	return s.summary.MarshalJSON()
}

// Insert inserts an item into the quantile summary
func (s *GKDistro) Insert(v float64, t SID) bool {
	e := GKEntry{
		V:       v,
		G:       1,
		Delta:   0,
		Samples: []SID{t},
	}

	eptr := s.summary.Insert(e)

	s.n++

	if eptr.prev[0] != s.summary.head && eptr.next[0] != nil {
		eptr.value.Delta = int(2 * s.eps * float64(s.n))
	}

	if s.n%int(1.0/float64(2.0*s.eps)) == 0 {
		s.compress()
	}

	// FIXME: choose if it needs to be sampled out?
	return true
}

func (s *GKDistro) compress() {
	var missing int

	epsN := int(2 * s.eps * float64(s.n))

	for elt := s.summary.head.next[0]; elt != nil && elt.next[0] != nil; {
		next := elt.next[0]
		t := elt.value
		nt := &next.value

		// value merging
		if t.V == nt.V {
			missing += nt.G
			nt.Delta += missing
			nt.G = t.G
			nt.Samples = append(nt.Samples, t.Samples...)
			s.summary.Remove(elt)
		} else if t.G+nt.G+missing+nt.Delta < epsN {
			nt.G += t.G + missing
			nt.Samples = append(nt.Samples, t.Samples...)
			missing = 0
			s.summary.Remove(elt)
		} else {
			nt.G += missing
			missing = 0
		}
		elt = next
	}
}

// Quantile returns an epsilon estimate of the element at quantile 'q' (0 <= q <= 1)
func (s *GKDistro) Quantile(q float64) (float64, []SID) {

	// convert quantile to rank
	r := int(q*float64(s.n) + 0.5)

	var rmin int
	epsN := int(s.eps * float64(s.n))

	for elt := s.summary.head.next[0]; elt != nil; elt = elt.next[0] {
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
	fmt.Printf("%sENTRY {v: %f, g: %d, delta:%d, tids: %v}\n", stroff, n.value.V, n.value.G, n.value.Delta, n.value.Samples)
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

// MarshalJSON returns a JSON representation
func (s *GKSkiplist) MarshalJSON() ([]byte, error) {
	// iterate over all available values and flatten the skiplist
	// FIXME: probably here we could allocate up to X if we compress before?
	var entries []GKEntry

	curr := s.head
	for curr != nil {
		entries = append(entries, curr.value)
		curr = curr.next[0]
	}

	return json.Marshal(entries)
}

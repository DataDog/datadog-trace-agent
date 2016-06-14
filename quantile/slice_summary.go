package quantile

import (
	"bytes"
	"fmt"
	"sort"
)

// SliceSummary is a GK-summary with a slice backend
type SliceSummary struct {
	Entries []Entry
	N       int
}

// NewSliceSummary allocates a new GK summary backed by a DLL
func NewSliceSummary() *SliceSummary {
	return &SliceSummary{}
}

func (s SliceSummary) String() string {
	var b bytes.Buffer
	b.WriteString("summary size: ")
	b.WriteString(fmt.Sprintf("%d", s.N))
	b.WriteRune('\n')

	gsum := 0

	for i, e := range s.Entries {
		gsum += e.G
		b.WriteString(fmt.Sprintf("v:%6.02f g:%05d d:%05d rmin:%05d rmax: %05d   ", e.V, e.G, e.Delta, gsum, gsum+e.Delta))
		if i%3 == 2 {
			b.WriteRune('\n')
		}
	}

	return b.String()
}

// Insert inserts a new value v in the summary paired with t (the ID of the span it was reported from)
func (s *SliceSummary) Insert(v float64, t uint64) {
	newEntry := Entry{
		V:       v,
		G:       1,
		Delta:   int(2 * EPSILON * float64(s.N)),
		Samples: []uint64{t},
	}

	i := sort.Search(len(s.Entries), func(i int) bool { return v < s.Entries[i].V })

	if i == 0 || i == len(s.Entries) {
		newEntry.Delta = 0
	}

	// allocate one more
	s.Entries = append(s.Entries, Entry{})
	copy(s.Entries[i+1:], s.Entries[i:])
	s.Entries[i] = newEntry
	s.N++

	if s.N%int(1.0/float64(2.0*EPSILON)) == 0 {
		s.compress()
	}
}

func (s *SliceSummary) compress() {
	epsN := int(2 * EPSILON * float64(s.N))

	var j, sum int
	for i := len(s.Entries) - 1; i >= 2; i = j - 1 {
		j = i - 1
		sum = s.Entries[j].G

		for j >= 1 && sum+s.Entries[i].G+s.Entries[i].Delta < epsN {
			j--
			sum += s.Entries[j].G
		}
		sum -= s.Entries[j].G
		j++

		if j < i {
			s.Entries[j].V = s.Entries[i].V
			s.Entries[j].G = sum + s.Entries[i].G
			s.Entries[j].Delta = s.Entries[i].Delta
			// copy the rest
			copy(s.Entries[j+1:], s.Entries[i+1:])
			// truncate to the numbers of removed elements
			s.Entries = s.Entries[:len(s.Entries)-(i-j)]
		}
	}
}

// Quantile returns an EPSILON estimate of the element at quantile 'q' (0 <= q <= 1)
func (s *SliceSummary) Quantile(q float64) (float64, []uint64) {
	// convert quantile to rank
	r := int(q*float64(s.N) + 0.5)

	var rmin int
	epsN := int(EPSILON * float64(s.N))

	for i := 0; i < len(s.Entries)-1; i++ {
		t := s.Entries[i]
		n := s.Entries[i+1]

		rmin += t.G

		if r+epsN < rmin+n.G+n.Delta {
			if r+epsN < rmin+n.G {
				return t.V, t.Samples
			}
			return n.V, n.Samples
		}
	}

	return s.Entries[len(s.Entries)-1].V, s.Entries[len(s.Entries)-1].Samples
}

// Merge two summaries entries together
func (s *SliceSummary) Merge(s2 *SliceSummary) {
	if s2.N == 0 {
		return
	}
	if s.N == 0 {
		s.N = s2.N
		s.Entries = make([]Entry, 0, len(s2.Entries))
		s.Entries = append(s.Entries, s2.Entries...)
		return
	}

	pos := 0
	end := len(s.Entries) - 1

	empties := make([]Entry, len(s2.Entries))
	s.Entries = append(s.Entries, empties...)

	for _, e := range s2.Entries {
		for pos <= end {
			if e.V > s.Entries[pos].V {
				pos++
				continue
			}
			copy(s.Entries[pos+1:end+2], s.Entries[pos:end+1])
			s.Entries[pos] = e
			pos++
			end++
			break
		}
		if pos > end {
			s.Entries[pos] = e
			pos++
		}
	}
	s.N += s2.N

	s.compress()
}

// Copy allocates a new summary with the same data
func (s *SliceSummary) Copy() *SliceSummary {
	s2 := NewSliceSummary()
	s2.Entries = make([]Entry, len(s.Entries))
	copy(s2.Entries, s.Entries)
	s2.N = s.N
	return s2
}

// BySlices returns a slice of Summary slices that represents weighted ranges of
// values
// e.g.    [0, 1]  : 3
//		   [1, 23] : 12 ...
// The number of intervals is related to the precision kept in the internal
// data structure to ensure epsilon*s.N precision on quantiles, but it's bounded.
// The weights are not exact, they're only upper bounds (see GK paper).
func (s *SliceSummary) BySlices(maxSamples int) []SummarySlice {
	var slices []SummarySlice

	var last float64
	for _, cur := range s.Entries {
		var sliceSamples []uint64
		if len(cur.Samples) > maxSamples {
			sliceSamples = cur.Samples[:maxSamples]
		} else {
			sliceSamples = cur.Samples
		}

		ss := SummarySlice{
			Start:   last,
			End:     cur.V,
			Weight:  cur.G,
			Samples: sliceSamples,
		}
		slices = append(slices, ss)

		last = cur.V
	}

	return slices
}

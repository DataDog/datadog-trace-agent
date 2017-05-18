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
		V:     v,
		G:     1,
		Delta: int(2 * EPSILON * float64(s.N)),
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
func (s *SliceSummary) Quantile(q float64) float64 {
	if len(s.Entries) == 0 {
		return 0
	}

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
				return t.V
			}
			return n.V
		}
	}

	return s.Entries[len(s.Entries)-1].V
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

type combinePtr struct {
	prev *Entry
	next *Entry
	this *Entry
	from int
}

func (s *SliceSummary) Combine(s2 *SliceSummary) {
	if s2.N == 0 {
		return
	}
	if s.N == 0 {
		s.N = s2.N
		s.Entries = make([]Entry, 0, len(s2.Entries))
		s.Entries = append(s.Entries, s2.Entries...)
		return
	}

	sorted := make([]*combinePtr, 0, len(s.Entries)+len(s2.Entries))

	var lastFrom1, lastFrom2 *combinePtr
	var pos1, pos2 int
	var len1, len2 int = len(s.Entries), len(s2.Entries)

	_ = "breakpoint"

	for pos1 < len1 || pos2 < len2 {
		fmt.Printf("pos1 %d, pos2 %d\n", pos1, pos2)
		fmt.Printf("pos1 %v, pos2 %v\n", s.Entries[pos1%len1], s.Entries[pos2%len2])
		if pos2 == len2 || (pos1 != len1 && s.Entries[pos1].V <= s2.Entries[pos2].V) {
			var last *Entry
			if lastFrom2 != nil {
				last = lastFrom2.this
			}

			cp := combinePtr{
				prev: last,
				this: &s.Entries[pos1],
				from: 1,
			}
			if lastFrom2 != nil {
				lastFrom2.next = &s.Entries[pos1]
			}
			sorted = append(sorted, &cp)
			lastFrom1 = &cp
			pos1++
		} else {
			var last *Entry
			if lastFrom1 != nil {
				last = lastFrom1.this
			}

			cp := combinePtr{
				prev: last,
				this: &s2.Entries[pos2],
				from: 2,
			}
			if lastFrom1 != nil {
				lastFrom1.next = &s2.Entries[pos2]
			}
			sorted = append(sorted, &cp)
			lastFrom2 = &cp
			pos2++
		}
	}

	newsumm := make([]Entry, 0, len(s.Entries)+len(s2.Entries))

	for _, cp := range sorted {
		fmt.Println(cp)
	}

	for _, cp := range sorted {
		newEntry := Entry{
			V: cp.this.V,
			G: cp.this.G,
		}

		var rmin, rmax int = 0, cp.this.Delta

		fmt.Println(cp)

		if cp.next != nil {
			rmax += cp.next.G + cp.next.Delta - 1
		} else {
			if cp.prev != nil {
				rmax += cp.prev.Delta
			}
		}

		newEntry.Delta = rmax - rmin

		newsumm = append(newsumm, newEntry)
	}

	s.Entries = newsumm
	s.N += s2.N
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
// When the bounds of the interval are equal, the weight is the number of times
// that exact value was inserted in the summary.
func (s *SliceSummary) BySlices() []SummarySlice {
	var slices []SummarySlice

	if len(s.Entries) == 0 {
		return slices
	}

	// by def in GK first val is always the min
	fs := SummarySlice{
		Start:  s.Entries[0].V,
		End:    s.Entries[0].V,
		Weight: 1,
	}
	slices = append(slices, fs)

	last := fs.End

	for _, cur := range s.Entries[1:] {
		lastSlice := &slices[len(slices)-1]
		if cur.V == lastSlice.Start && cur.V == lastSlice.End {
			lastSlice.Weight += cur.G
			continue
		}

		if cur.G == 1 {
			last = cur.V
		}

		ss := SummarySlice{
			Start:  last,
			End:    cur.V,
			Weight: cur.G,
		}
		slices = append(slices, ss)

		last = cur.V
	}

	return slices
}

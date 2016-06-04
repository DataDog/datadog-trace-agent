package quantile

import "container/list"

// SimpleSummary is a GK-summary with a slice backend
type SimpleSummary struct {
	Entries *list.List
	N       int
}

// NewSimpleSummary allocates a new GK summary backed by a DLL
func NewSimpleSummary() *SimpleSummary {
	return &SimpleSummary{
		Entries: list.New(),
	}
}

// Insert inserts a new value v in the summary paired with t (the ID of the span it was reported from)
func (s *SimpleSummary) Insert(v float64, t uint64) {
	newEntry := Entry{
		V:       v,
		G:       1,
		Delta:   int(2 * EPSILON * float64(s.N)),
		Samples: []uint64{t},
	}

	var pos *list.Element
	for pos = s.Entries.Front(); pos != nil; pos = pos.Next() {
		elt := pos.Value.(Entry)
		if v < elt.V {
			break
		}
	}

	if pos == s.Entries.Back() || pos == s.Entries.Front() || s.Entries.Len() == 0 {
		newEntry.Delta = 0
	}

	// allocate one more
	if s.Entries.Len() == 0 || pos == nil {
		s.Entries.PushBack(newEntry)
	} else {
		s.Entries.InsertBefore(newEntry, pos)
	}
	s.N++

	if s.N%int(1.0/float64(2.0*EPSILON)) == 0 {
		s.compress()
	}
}

func (s *SimpleSummary) compress() {
	var missing int

	epsN := int(2 * EPSILON * float64(s.N))

	for pos := s.Entries.Front(); pos != s.Entries.Back(); {
		elt := pos.Value.(Entry)
		nt := pos.Next()
		next := nt.Value.(Entry)

		// value merging
		if elt.V == next.V {
			missing += next.G
			next.Delta += missing
			next.G = elt.G
			s.Entries.Remove(pos)
		} else if elt.G+next.G+missing+next.Delta < epsN {
			next.G += elt.G + missing
			missing = 0
			s.Entries.Remove(pos)
		} else {
			next.G += missing
			missing = 0
		}

		pos = nt
	}
}

// Quantile returns an EPSILON estimate of the element at quantile 'q' (0 <= q <= 1)
func (s *SimpleSummary) Quantile(q float64) (float64, []uint64) {

	// convert quantile to rank
	r := int(q*float64(s.N) + 0.5)

	var rmin int
	epsN := int(EPSILON * float64(s.N))

	for pos := s.Entries.Front(); pos != nil; pos = pos.Next() {
		t := pos.Value.(Entry)

		rmin += t.G

		if pos.Next() == nil {
			return t.V, t.Samples
		}

		n := pos.Next().Value.(Entry)

		if r+epsN < rmin+n.G+n.Delta {
			if r+epsN < rmin+n.G {
				return t.V, t.Samples
			}
			return n.V, n.Samples
		}
	}

	panic("not reached")
}

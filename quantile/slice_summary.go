package quantile

// SliceSummary is a GK-summary with a slice backend
type SliceSummary struct {
	Entries []Entry
	N       int
}

// NewSliceSummary allocates a new GK summary backed by a DLL
func NewSliceSummary() *SliceSummary {
	return &SliceSummary{}
}

// Insert inserts a new value v in the summary paired with t (the ID of the span it was reported from)
func (s *SliceSummary) Insert(v float64, t uint64) {
	newEntry := Entry{
		V:       v,
		G:       1,
		Delta:   int(2 * EPSILON * float64(s.N)),
		Samples: []uint64{t},
	}

	var i int
	for _, e := range s.Entries {
		if v < e.V {
			break
		}
		i++
	}

	if i == 0 || i == len(s.Entries)-1 {
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
	var missing int

	epsN := int(2 * EPSILON * float64(s.N))

	for i := 0; i < len(s.Entries)-1; i++ {
		elt := s.Entries[i]
		next := s.Entries[i+1]

		// value merging
		if elt.V == next.V {
			missing += next.G
			next.Delta += missing
			next.G = elt.G
			copy(s.Entries[i:], s.Entries[i+1:])
			s.Entries[len(s.Entries)-1] = Entry{}
			s.Entries = s.Entries[:len(s.Entries)-1]
		} else if elt.G+next.G+missing+next.Delta < epsN {
			next.G += elt.G + missing
			missing = 0
			copy(s.Entries[i:], s.Entries[i+1:])
			s.Entries[len(s.Entries)-1] = Entry{}
			s.Entries = s.Entries[:len(s.Entries)-1]
		} else {
			next.G += missing
			missing = 0
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

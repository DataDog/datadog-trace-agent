package model

// FloatBitsSlice is used to sort IEEE Float64's as bits
type FloatBitsSlice []uint64

func (s FloatBitsSlice) Len() int { return len(s) }
func (s FloatBitsSlice) Less(i, j int) bool {
	sgni := s[i] >> 63
	sgnj := s[j] >> 63

	// both positive, then 'less than' is correct
	if sgni == sgnj && sgni == 0 {
		return s[i] < s[j]
	}

	// Either the signs differ, or they're both negative.  If we have one
	// positive and one negative, then the negative number which sorts
	// higher than a positive number (due to the sign bit) we actually want
	// to tag as 'less than'.  If they're both negative, then the 'larger'
	// uint64 value is the 'smaller' float64

	return s[i] >= s[j]

}
func (s FloatBitsSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

package quantile

import (
	"fmt"
	"testing"
)

func TestBySlicesWeightedHalf(t *testing.T) {
	s := NewSliceSummary()
	for i := 0; i < 100000; i++ {
		s.Insert(float64(i%10000), 0)
	}

	s2 := NewSliceSummary()
	for i := 0; i < 100000; i++ {
		s2.Insert(float64(i%10000), 0)
	}

	sw1 := WeightedSliceSummary{1.0, s}
	sw2 := WeightedSliceSummary{0.5, s2}

	ss := BySlicesWeighted(sw1, sw2)
	total := 0
	for _, sl := range ss {
		total += sl.Weight
	}

	fmt.Println(ss)
	fmt.Println(total)
}

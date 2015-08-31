package main

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

const EPSILON float64 = 1e-2

func newTestSpan(tid, sid uint64) model.Span {
	return model.Span{TraceID: tid, SpanID: sid}
}

// Will insert nothing and return a given array of ids and random value for Quantile()
type FakeSummary struct {
	FakeQuantiles map[float64][]uint64
}

func NewFakeSummary() *FakeSummary {
	return &FakeSummary{FakeQuantiles: make(map[float64][]uint64)}
}
func (s *FakeSummary) FakeQuantile(q float64, sids []uint64) { s.FakeQuantiles[q] = sids }
func (s *FakeSummary) Insert(v int64, t uint64)              {}
func (s *FakeSummary) Quantile(q float64) (int64, []uint64) {
	quantile := rand.Int63()
	vals, ok := s.FakeQuantiles[q]
	if !ok {
		return quantile, []uint64{}
	}
	return quantile, vals
}

func TestSampler(t *testing.T) {
	assert := assert.New(t)

	sampler := NewSampler()
	assert.True(sampler.IsEmpty())

	// Add some spans to our sampler
	testSpans := []model.Span{
		newTestSpan(0, 0),
		newTestSpan(0, 1),
		newTestSpan(0, 2),
		newTestSpan(1, 3),
		newTestSpan(0, 4),
		newTestSpan(2, 5),
		newTestSpan(3, 6),
		newTestSpan(3, 7),
		newTestSpan(1, 8),
		newTestSpan(4, 9),
	}
	for _, s := range testSpans {
		sampler.AddSpan(s)
	}

	// Now prepare distributions
	stats := model.NewStatsBucket(EPSILON)
	tgs := model.NewTagsFromString("service:dogweb,resource:dash.list")
	faked := model.NewDistribution(model.DURATION, tgs, 0)
	fakes := NewFakeSummary()
	faked.Summary = fakes
	fakes.FakeQuantile(0, []uint64{5})
	fakes.FakeQuantile(0.5, []uint64{2})
	fakes.FakeQuantile(1, []uint64{3, 8}) // supposed to pick only the first one

	// Add one fake distribution for choosing
	stats.Distributions["whatever"] = faked
	chosen := sampler.GetSamples(stats, []float64{0, 0.5, 1})

	shouldChoose := []int{0, 1, 2, 3, 4, 5, 8}
	chosenSID := make([]int, len(chosen))
	for i, s := range chosen {
		chosenSID[i] = int(s.SpanID)
	}
	sort.Ints(chosenSID)

	// Verify that are our samples are correct
	assert.Equal(shouldChoose, chosenSID)
}

package main

import (
	"sort"
	"testing"

	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func TestSampler(t *testing.T) {
	assert := assert.New(t)

	sampler := NewSampler()
	assert.True(sampler.IsEmpty())

	// Add some spans to our sampler
	testSpans := []model.Span{
		model.Span{TraceID: 100, SpanID: 10, Duration: 573496},
		model.Span{TraceID: 100, SpanID: 11, Duration: 992312},
		model.Span{TraceID: 100, SpanID: 12, Duration: 769540},
		model.Span{TraceID: 101, SpanID: 13, Duration: 26965},
		model.Span{TraceID: 100, SpanID: 14, Duration: 513323},
		model.Span{TraceID: 102, SpanID: 15, Duration: 34347},
		model.Span{TraceID: 103, SpanID: 16, Duration: 498798},
		model.Span{TraceID: 103, SpanID: 17, Duration: 19207},
		model.Span{TraceID: 101, SpanID: 18, Duration: 197884},
		model.Span{TraceID: 104, SpanID: 19, Duration: 151384},
	}
	for _, s := range testSpans {
		sampler.AddSpan(s)
	}

	// Now prepare distributions
	stats := model.NewStatsBucket(0, 1)
	tgs := model.NewTagsFromString("service:dogweb,resource:dash.list")
	d := model.NewDistribution(model.DURATION, tgs)
	for _, span := range testSpans {
		d.Add(span)
	}

	// not too good as we also test the quantile package
	// but if these asserts pass we're sure we're correctly testing the sampler
	assert.Equal(len(testSpans), d.Summary.N)

	q1, samp1 := d.Summary.Quantile(0.5)
	assert.Equal(int64(197884), q1)
	assert.Equal(1, len(samp1))
	assert.Equal(uint64(18), samp1[0])

	q2, samp2 := d.Summary.Quantile(0.90)
	assert.Equal(int64(769540), q2)
	assert.Equal(1, len(samp2))
	assert.Equal(uint64(12), samp2[0])

	q3, samp3 := d.Summary.Quantile(0.99)
	assert.Equal(int64(992312), q3)
	assert.Equal(1, len(samp3))
	assert.Equal(uint64(11), samp3[0])

	// Add one fake distribution for choosing
	stats.Distributions["whatever"] = d
	chosen := sampler.GetSamples(stats, []float64{0.5, 0.90, 0.99})

	shouldChoose := []int{10, 11, 12, 13, 14, 18}
	chosenSID := make([]int, len(chosen))
	for i, s := range chosen {
		chosenSID[i] = int(s.SpanID)
	}
	sort.Ints(chosenSID)

	// Verify that are our samples are correct
	assert.Equal(shouldChoose, chosenSID)
}

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
		model.Span{TraceID: 0, SpanID: 0, Duration: 573496},
		model.Span{TraceID: 0, SpanID: 1, Duration: 992312},
		model.Span{TraceID: 0, SpanID: 2, Duration: 769540},
		model.Span{TraceID: 1, SpanID: 3, Duration: 26965},
		model.Span{TraceID: 0, SpanID: 4, Duration: 513323},
		model.Span{TraceID: 2, SpanID: 5, Duration: 34347},
		model.Span{TraceID: 3, SpanID: 6, Duration: 498798},
		model.Span{TraceID: 3, SpanID: 7, Duration: 19207},
		model.Span{TraceID: 1, SpanID: 8, Duration: 197884},
		model.Span{TraceID: 4, SpanID: 9, Duration: 151384},
	}
	for _, s := range testSpans {
		sampler.AddSpan(s)
	}

	// Now prepare distributions
	stats := model.NewStatsBucket(0)
	tgs := model.NewTagsFromString("service:dogweb,resource:dash.list")
	d := model.NewDistribution(model.DURATION, tgs)
	for _, span := range testSpans {
		d.Add(span)
	}

	// not too good as we also test the quantile package
	// but if these asserts pass we're sure we're correctly testing the sampler
	assert.Equal(len(testSpans), d.Summary.N)

	q1, samp1 := d.Summary.Quantile(0.5)
	assert.Equal(197884, q1)
	assert.Equal(1, len(samp1))
	assert.Equal(uint64(8), samp1[0])

	q2, samp2 := d.Summary.Quantile(0.90)
	assert.Equal(769540, q2)
	assert.Equal(1, len(samp2))
	assert.Equal(uint64(2), samp2[0])

	q3, samp3 := d.Summary.Quantile(0.99)
	assert.Equal(992312, q3)
	assert.Equal(1, len(samp3))
	assert.Equal(uint64(1), samp3[0])

	// Add one fake distribution for choosing
	stats.Distributions["whatever"] = d
	chosen := sampler.GetSamples(stats, []float64{0.5, 0.90, 0.99})

	shouldChoose := []int{0, 1, 2, 3, 4, 8}
	chosenSID := make([]int, len(chosen))
	for i, s := range chosen {
		chosenSID[i] = int(s.SpanID)
	}
	sort.Ints(chosenSID)

	// Verify that are our samples are correct
	assert.Equal(shouldChoose, chosenSID)
}

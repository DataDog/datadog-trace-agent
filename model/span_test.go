package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testSpan() Span {
	return Span{
		Duration: 10000000,
		Error:    0,
		Resource: "GET /some/raclette",
		Service:  "django",
		Name:     "django.controller",
		SpanID:   42,
		Start:    1448466874000000000,
		TraceID:  424242,
		Meta: map[string]string{
			"user": "leo",
			"pool": "fondue",
		},
		Metrics: map[string]float64{
			"cheese_weight": 100000.0,
		},
		ParentID: 1111,
		Type:     "http",
	}
}

func TestSpanString(t *testing.T) {
	assert := assert.New(t)
	assert.NotEqual("", testSpan().String())
}

func TestSpanFlushMarker(t *testing.T) {
	assert := assert.New(t)
	s := NewFlushMarker()
	assert.True(s.IsFlushMarker())
}

func TestSpanWeight(t *testing.T) {
	assert := assert.New(t)

	span := testSpan()
	assert.Equal(1.0, span.Weight())

	span.Metrics[SpanSampleRateMetricKey] = -1.0
	assert.Equal(1.0, span.Weight())

	span.Metrics[SpanSampleRateMetricKey] = 0.0
	assert.Equal(1.0, span.Weight())

	span.Metrics[SpanSampleRateMetricKey] = 0.25
	assert.Equal(4.0, span.Weight())

	span.Metrics[SpanSampleRateMetricKey] = 1.0
	assert.Equal(1.0, span.Weight())

	span.Metrics[SpanSampleRateMetricKey] = 1.5
	assert.Equal(1.0, span.Weight())
}

func TestSpansCoveredDuration(t *testing.T) {
	assert := assert.New(t)

	span := func(start, duration int64) *Span {
		return &Span{
			Start:    start,
			Duration: duration,
		}
	}

	tests := []struct {
		parentStart int64
		spans       Spans
		duration    int64
	}{
		{0, Spans{}, 0},
		{0, Spans{span(0, 100)}, 100},
		{0, Spans{span(0, 50), span(50, 50)}, 100},
		{0, Spans{span(0, 50), span(10, 20)}, 50},
		{0, Spans{span(10, 20), span(0, 30)}, 30},
		{0, Spans{span(10, 20), span(50, 20)}, 40},
		{0, Spans{span(10, 20), span(15, 20)}, 25},
		{0, Spans{span(10, 20), span(5, 30), span(50, 10)}, 40},

		{5, Spans{span(10, 10), span(15, 10)}, 15},
		{5, Spans{span(0, 10)}, 5},
		{5, Spans{span(0, 10), span(5, 10)}, 10},

		{40, Spans{span(0, 60), span(10, 10), span(30, 10)}, 20},
	}

	for _, test := range tests {
		coveredDuration := test.spans.CoveredDuration(test.parentStart)
		assert.Equal(test.duration, coveredDuration,
			fmt.Sprintf("%#v", test.spans))
	}
}

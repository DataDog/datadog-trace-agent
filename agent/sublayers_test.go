package main

import (
	"testing"
	"time"

	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func TestSublayerNested(t *testing.T) {
	assert := assert.New(t)

	// real trace
	tr := model.Trace{
		model.Span{TraceID: 1, SpanID: 1, ParentID: 0, Start: 42, Duration: 1000000000, Service: "mcnulty", Type: "web"},
		model.Span{TraceID: 1, SpanID: 2, ParentID: 1, Start: 100, Duration: 200000000, Service: "mcnulty", Type: "sql"},
		model.Span{TraceID: 1, SpanID: 3, ParentID: 2, Start: 150, Duration: 199999000, Service: "master-db", Type: "sql"},
		model.Span{TraceID: 1, SpanID: 4, ParentID: 1, Start: 500000000, Duration: 500000, Service: "redis", Type: "redis"},
		model.Span{TraceID: 1, SpanID: 5, ParentID: 1, Start: 700000000, Duration: 700000, Service: "mcnulty", Type: ""},
	}

	in := make(chan model.Trace)
	st := NewSublayerTagger(in)
	st.Start()

	timeout := make(chan struct{}, 1)
	go func() {
		time.Sleep(time.Second)
		timeout <- struct{}{}
	}()

	in <- tr
	var result model.Trace
	select {
	case <-timeout:
		t.Fatal("did not receive sublayers tagged trace in time")
	case result = <-st.out:
	}

	// assert sublayers result
	for _, s := range result {
		if s.ParentID == 0 {
			assert.Equal(
				s.Metrics,
				map[string]float64{
					"_sublayers.span_count":          5,
					"_sublayers.byType.web":          1000000000 - 200000000 - 500000,
					"_sublayers.byType.sql":          200000000,
					"_sublayers.byType.redis":        500000,
					"_sublayers.byService.mcnulty":   1000000000 - 199999000 - 500000,
					"_sublayers.byService.master-db": 199999000,
					"_sublayers.byService.redis":     500000,
				},
				"did not get exepected sublayers tagging",
			)

		} else {
			assert.Nil(s.Metrics)
		}
	}
}

func BenchmarkSublayerThru(b *testing.B) {
	// real trace
	tr := model.Trace{
		model.Span{TraceID: 1, SpanID: 1, ParentID: 0, Start: 42, Duration: 1000000000, Service: "mcnulty", Type: "web"},
		model.Span{TraceID: 1, SpanID: 2, ParentID: 1, Start: 100, Duration: 200000000, Service: "mcnulty", Type: "sql"},
		model.Span{TraceID: 1, SpanID: 3, ParentID: 2, Start: 150, Duration: 199999000, Service: "master-db", Type: "sql"},
		model.Span{TraceID: 1, SpanID: 4, ParentID: 1, Start: 500000000, Duration: 500000, Service: "redis", Type: "redis"},
		model.Span{TraceID: 1, SpanID: 5, ParentID: 1, Start: 700000000, Duration: 700000, Service: "mcnulty", Type: ""},
	}

	in := make(chan model.Trace)
	st := NewSublayerTagger(in)
	st.Start()
	go func() {
		for _ = range st.out {
		}
	}()

	for i := 0; i < b.N; i++ {
		in <- tr
	}
}

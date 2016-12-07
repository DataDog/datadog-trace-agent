package main

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestSublayerNested(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sublayer nested test in Short mode.")
	}

	assert := assert.New(t)

	waitBucket(t, testBucketInterval)

	// real trace
	now := time.Now().UnixNano()
	tr := model.Trace{
		model.Span{TraceID: 1, SpanID: 1, ParentID: 0, Start: now + 42, Duration: 1000000000, Service: "mcnulty", Type: "web"},
		model.Span{TraceID: 1, SpanID: 2, ParentID: 1, Start: now + 100, Duration: 200000000, Service: "mcnulty", Type: "sql"},
		model.Span{TraceID: 1, SpanID: 3, ParentID: 2, Start: now + 150, Duration: 199999000, Service: "master-db", Type: "sql"},
		model.Span{TraceID: 1, SpanID: 4, ParentID: 1, Start: now + 500000000, Duration: 500000, Service: "redis", Type: "redis"},
		model.Span{TraceID: 1, SpanID: 5, ParentID: 1, Start: now + 700000000, Duration: 700000, Service: "mcnulty", Type: ""},
	}

	in := make(chan model.Trace)
	st := NewSublayerTagger(in)
	go st.Run()

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
					"_sublayers.span_count":                                     5,
					"_sublayers.duration.by_type.sublayer_type:web":             1000000000 - 200000000 - 500000,
					"_sublayers.duration.by_type.sublayer_type:sql":             200000000,
					"_sublayers.duration.by_type.sublayer_type:redis":           500000,
					"_sublayers.duration.by_service.sublayer_service:mcnulty":   1000000000 - 199999000 - 500000,
					"_sublayers.duration.by_service.sublayer_service:master-db": 199999000,
					"_sublayers.duration.by_service.sublayer_service:redis":     500000,
				},
				"did not get exepected sublayers tagging",
			)

		} else {
			assert.Nil(s.Metrics)
		}
	}

	// pass that trace into the concentrator and assert sublayer counts
	c := NewTestConcentrator()
	c.conf.BucketInterval = testBucketInterval
	c.conf.OldestSpanCutoff = c.conf.BucketInterval.Nanoseconds()

	go c.Run()

	c.in <- result

	time.Sleep(c.conf.BucketInterval)

	// now flush the concentrator
	c.in <- model.NewTraceFlushMarker()

	time.Sleep(3 * c.conf.BucketInterval)

	ctimeout := make(chan struct{}, 1)
	go func() {
		time.Sleep(time.Second)
		ctimeout <- struct{}{}
	}()

	var stats []model.StatsBucket
	select {
	case <-ctimeout:
		t.Fatal("did not receive stats from the concentrator in time")
	case stats = <-c.out:
	}

	expected := map[string]model.Count{
		"|_sublayers.duration.by_service|env:none,service:mcnulty,sublayer_service:master-db": model.Count{
			Key:     "|_sublayers.duration.by_service|env:none,service:mcnulty,sublayer_service:master-db",
			Measure: "_sublayers.duration.by_service",
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty,sublayer_service:master-db"),
			Value:   199999000,
		},
		"|_sublayers.duration.by_service|env:none,service:mcnulty,sublayer_service:mcnulty": model.Count{
			Key:     "|_sublayers.duration.by_service|env:none,service:mcnulty,sublayer_service:mcnulty",
			Measure: "_sublayers.duration.by_service",
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty,sublayer_service:mcnulty"),
			Value:   1000000000 - 199999000 - 500000,
		},
		"|_sublayers.duration.by_service|env:none,service:mcnulty,sublayer_service:redis": model.Count{
			Key:     "|_sublayers.duration.by_service|env:none,service:mcnulty,sublayer_service:redis",
			Measure: "_sublayers.duration.by_service",
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty,sublayer_service:redis"),
			Value:   500000,
		},
		"|_sublayers.duration.by_type|env:none,service:mcnulty,sublayer_type:redis": model.Count{
			Key:     "|_sublayers.duration.by_type|env:none,service:mcnulty,sublayer_type:redis",
			Measure: "_sublayers.duration.by_type",
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty,sublayer_type:redis"),
			Value:   500000,
		},
		"|_sublayers.duration.by_type|env:none,service:mcnulty,sublayer_type:sql": model.Count{
			Key:     "|_sublayers.duration.by_type|env:none,service:mcnulty,sublayer_type:sql",
			Measure: "_sublayers.duration.by_type",
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty,sublayer_type:sql"),
			Value:   200000000,
		},
		"|_sublayers.duration.by_type|env:none,service:mcnulty,sublayer_type:web": model.Count{
			Key:     "|_sublayers.duration.by_type|env:none,service:mcnulty,sublayer_type:web",
			Measure: "_sublayers.duration.by_type",
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty,sublayer_type:web"),
			Value:   1000000000 - 200000000 - 500000,
		},
		"|duration|env:none,service:master-db": model.Count{
			Key:     "|duration|env:none,service:master-db",
			Measure: model.DURATION,
			TagSet:  model.NewTagSetFromString("env:none,service:master-db"),
			Value:   199999000,
		},
		"|duration|env:none,service:mcnulty": model.Count{
			Key:     "|duration|env:none,service:mcnulty",
			Measure: model.DURATION,
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty"),
			Value:   1000000000 + 200000000 + 700000,
		},
		"|duration|env:none,service:redis": model.Count{
			Key:     "|duration|env:none,service:redis",
			Measure: model.DURATION,
			TagSet:  model.NewTagSetFromString("env:none,service:redis"),
			Value:   500000,
		},
		"|errors|env:none,service:master-db": model.Count{
			Key:     "|errors|env:none,service:master-db",
			Measure: model.ERRORS,
			TagSet:  model.NewTagSetFromString("env:none,service:master-db"),
			Value:   0,
		},
		"|errors|env:none,service:mcnulty": model.Count{
			Key:     "|errors|env:none,service:mcnulty",
			Measure: model.ERRORS,
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty"),
			Value:   0,
		},
		"|errors|env:none,service:redis": model.Count{
			Key:     "|errors|env:none,service:redis",
			Measure: model.ERRORS,
			TagSet:  model.NewTagSetFromString("env:none,service:redis"),
			Value:   0,
		},
		"|hits|env:none,service:master-db": model.Count{
			Key:     "|hits|env:none,service:master-db",
			Measure: model.HITS,
			TagSet:  model.NewTagSetFromString("env:none,service:master-db"),
			Value:   1,
		},
		"|hits|env:none,service:mcnulty": model.Count{
			Key:     "|hits|env:none,service:mcnulty",
			Measure: model.HITS,
			TagSet:  model.NewTagSetFromString("env:none,service:mcnulty"),
			Value:   3,
		},
		"|hits|env:none,service:redis": model.Count{
			Key:     "|hits|env:none,service:redis",
			Measure: model.HITS,
			TagSet:  model.NewTagSetFromString("env:none,service:redis"),
			Value:   1,
		},
	}

	assert.Equal(len(expected), len(stats[0].Counts), "got %v", stats[0].Counts)

	t.Logf("stats[0].Counts: %v", stats[0].Counts)
	for k, c := range stats[0].Counts {
		exp, ok := expected[k]
		assert.True(ok, "count not expected %v", c)

		assert.Equal(exp, c)
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

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tagSublayers(tr)
	}
}

package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testSpans = []Span{
	Span{Service: "A", Name: "A.foo", Resource: "α", Duration: 1},
	Span{Service: "A", Name: "A.foo", Resource: "β", Duration: 2, Error: 1},
	Span{Service: "B", Name: "B.foo", Resource: "γ", Duration: 3},
	Span{Service: "B", Name: "B.foo", Resource: "ε", Duration: 4, Error: 404},
	Span{Service: "B", Name: "B.foo", Resource: "ζ", Duration: 5, Meta: map[string]string{"version": "1.3"}},
	Span{Service: "B", Name: "sql.query", Resource: "ζ", Duration: 6, Meta: map[string]string{"version": "1.4"}},
	Span{Service: "C", Name: "sql.query", Resource: "δ", Duration: 7},
	Span{Service: "C", Name: "sql.query", Resource: "δ", Duration: 8},
}

func TestStatsBucketDefault(t *testing.T) {
	assert := assert.New(t)

	sb := NewStatsBucket(0, 1e9, time.Millisecond)
	aggr := []string{"service", "name", "resource"}
	for _, s := range testSpans {
		sb.HandleSpan(s, aggr)
	}

	expectedCounts := map[string]int64{
		"duration|name:A.foo,resource:α,service:A":     1,
		"duration|name:A.foo,resource:β,service:A":     2,
		"duration|name:B.foo,resource:γ,service:B":     3,
		"duration|name:B.foo,resource:ε,service:B":     4,
		"duration|name:B.foo,resource:ζ,service:B":     5,
		"duration|name:sql.query,resource:ζ,service:B": 6,
		"duration|name:sql.query,resource:δ,service:C": 15,
		"errors|name:A.foo,resource:α,service:A":       0,
		"errors|name:A.foo,resource:β,service:A":       1,
		"errors|name:B.foo,resource:γ,service:B":       0,
		"errors|name:B.foo,resource:ε,service:B":       1,
		"errors|name:B.foo,resource:ζ,service:B":       0,
		"errors|name:sql.query,resource:ζ,service:B":   0,
		"errors|name:sql.query,resource:δ,service:C":   0,
		"hits|name:A.foo,resource:α,service:A":         1,
		"hits|name:A.foo,resource:β,service:A":         1,
		"hits|name:B.foo,resource:γ,service:B":         1,
		"hits|name:B.foo,resource:ε,service:B":         1,
		"hits|name:B.foo,resource:ζ,service:B":         1,
		"hits|name:sql.query,resource:ζ,service:B":     1,
		"hits|name:sql.query,resource:δ,service:C":     2,
	}

	assert.Len(sb.Counts, len(expectedCounts), "Missing counts!")
	for ckey, c := range sb.Counts {
		val, ok := expectedCounts[ckey]
		if !ok {
			assert.Fail("Unexpected count %s", ckey)
		}
		assert.Equal(val, c.Value, "Count %s wrong value", ckey)
	}
}

func TestStatsBucketExtraAggregators(t *testing.T) {
	assert := assert.New(t)

	sb := NewStatsBucket(0, 1e9, time.Millisecond)
	aggr := []string{"name", "version"}
	for _, s := range testSpans {
		sb.HandleSpan(s, aggr)
	}

	expectedCounts := map[string]int64{
		"duration|name:A.foo":                 3,
		"duration|name:B.foo":                 7,
		"duration|name:B.foo,version:1.3":     5,
		"duration|name:sql.query":             15,
		"duration|name:sql.query,version:1.4": 6,
		"errors|name:A.foo":                   1,
		"errors|name:B.foo":                   1,
		"errors|name:B.foo,version:1.3":       0,
		"errors|name:sql.query":               0,
		"errors|name:sql.query,version:1.4":   0,
		"hits|name:A.foo":                     2,
		"hits|name:B.foo":                     2,
		"hits|name:B.foo,version:1.3":         1,
		"hits|name:sql.query":                 2,
		"hits|name:sql.query,version:1.4":     1,
	}

	assert.Len(sb.Counts, len(expectedCounts), "Missing counts!")
	for ckey, c := range sb.Counts {
		val, ok := expectedCounts[ckey]
		if !ok {
			assert.Fail("Unexpected count %s", ckey)
		}
		assert.Equal(val, c.Value, "Count %s wrong value", ckey)
	}
}

func TestResos(t *testing.T) {
	assert := assert.New(t)

	durations := []int64{
		3 * 1e9,
		32432874923,
		1000,
		45,
		41000234,
	}

	type testcase struct {
		res time.Duration
		exp []float64
	}

	cases := []testcase{
		{time.Second, []float64{3000000000, 32000000000, 0, 0, 0}},
		{time.Millisecond, []float64{3000000000, 32432000000, 0, 0, 41000000}},
		{time.Microsecond, []float64{3000000000, 32432874000, 1000, 0, 41000000}},
		{time.Nanosecond, []float64{3000000000, 32432874923, 1000, 45, 41000234}},
	}

	for _, c := range cases {
		results := []float64{}
		for _, d := range durations {
			results = append(results, nsTimestampToFloat(d, c.res))
		}

		assert.Equal(c.exp, results, "resolution conversion failed")
	}
}

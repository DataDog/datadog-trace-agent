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

func TestGrainKey(t *testing.T) {
	assert := assert.New(t)
	gk := GrainKey("serve", "duration", "service:webserver")
	assert.Equal("serve|duration|service:webserver", gk)
}

func TestStatsBucketDefault(t *testing.T) {
	assert := assert.New(t)

	sb := NewStatsBucket(0, 1e9)

	// No custom aggregators only the defaults
	aggr := []string{}
	for _, s := range testSpans {
		t.Logf("s: %v", s)
		t.Logf("sb: %v", sb)
		//sb.HandleSpan(s, aggr)
		aggrString, tgs := getAggregateGrain(s, aggr, &sb.keyBuf)
		t.Logf("aggrString: %s", aggrString)
		sb.addToTagSet(s, aggrString, tgs)
	}

	expectedCounts := map[string]int64{
		"A.foo|duration|resource:α,service:A":     1,
		"A.foo|duration|resource:β,service:A":     2,
		"B.foo|duration|resource:γ,service:B":     3,
		"B.foo|duration|resource:ε,service:B":     4,
		"B.foo|duration|resource:ζ,service:B":     5,
		"sql.query|duration|resource:ζ,service:B": 6,
		"sql.query|duration|resource:δ,service:C": 15,
		"A.foo|errors|resource:α,service:A":       0,
		"A.foo|errors|resource:β,service:A":       1,
		"B.foo|errors|resource:γ,service:B":       0,
		"B.foo|errors|resource:ε,service:B":       1,
		"B.foo|errors|resource:ζ,service:B":       0,
		"sql.query|errors|resource:ζ,service:B":   0,
		"sql.query|errors|resource:δ,service:C":   0,
		"A.foo|hits|resource:α,service:A":         1,
		"A.foo|hits|resource:β,service:A":         1,
		"B.foo|hits|resource:γ,service:B":         1,
		"B.foo|hits|resource:ε,service:B":         1,
		"B.foo|hits|resource:ζ,service:B":         1,
		"sql.query|hits|resource:ζ,service:B":     1,
		"sql.query|hits|resource:δ,service:C":     2,
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

	sb := NewStatsBucket(0, 1e9)

	// one custom aggregator
	aggr := []string{"version"}
	for _, s := range testSpans {
		sb.HandleSpan(s, aggr)
	}

	expectedCounts := map[string]int64{
		"A.foo|duration|resource:α,service:A":                 1,
		"A.foo|duration|resource:β,service:A":                 2,
		"B.foo|duration|resource:γ,service:B":                 3,
		"B.foo|duration|resource:ε,service:B":                 4,
		"sql.query|duration|resource:δ,service:C":             15,
		"A.foo|errors|resource:α,service:A":                   0,
		"A.foo|errors|resource:β,service:A":                   1,
		"B.foo|errors|resource:γ,service:B":                   0,
		"B.foo|errors|resource:ε,service:B":                   1,
		"sql.query|errors|resource:δ,service:C":               0,
		"A.foo|hits|resource:α,service:A":                     1,
		"A.foo|hits|resource:β,service:A":                     1,
		"B.foo|hits|resource:γ,service:B":                     1,
		"B.foo|hits|resource:ε,service:B":                     1,
		"sql.query|hits|resource:δ,service:C":                 2,
		"sql.query|errors|resource:ζ,service:B,version:1.4":   0,
		"sql.query|hits|resource:ζ,service:B,version:1.4":     1,
		"sql.query|duration|resource:ζ,service:B,version:1.4": 6,
		"B.foo|errors|resource:ζ,service:B,version:1.3":       0,
		"B.foo|duration|resource:ζ,service:B,version:1.3":     5,
		"B.foo|hits|resource:ζ,service:B,version:1.3":         1,
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

func TestTsRounding(t *testing.T) {
	assert := assert.New(t)

	durations := []int64{
		3 * 1e9,     // 10110010110100000101111000000000 -> 10110010110000000000000000000000 = 2998927360
		32432874923, // 11110001101001001100110010110101011 -> 11110001100000000000000000000000000 = 32413581312
		1000,        // Keep it with full precision
		45,          // Keep it with full precision
		41000234,    // 10011100011001110100101010 -> 10011100010000000000000000 = 40960000
	}

	type testcase struct {
		res time.Duration
		exp []float64
	}

	exp := []float64{2998927360, 32413581312, 1000, 45, 40960000}

	results := []float64{}
	for _, d := range durations {
		results = append(results, nsTimestampToFloat(d))
	}
	assert.Equal(exp, results, "Unproper rounding of timestamp")
}

func BenchmarkHandleSpan(b *testing.B) {
	sb := NewStatsBucket(0, 1e9)
	aggr := []string{}
	for i := 0; i < b.N; i++ {
		for _, s := range testSpans {
			sb.HandleSpan(s, aggr)
		}
	}
	b.ReportAllocs()
}

func BenchmarkGetAggregateString(b *testing.B) {
	aggr := []string{}
	sb := NewStatsBucket(0, 1e9)
	for i := 0; i < b.N; i++ {
		for _, s := range testSpans {
			getAggregateGrain(s, aggr, &sb.keyBuf)
		}
	}
	b.ReportAllocs()
}

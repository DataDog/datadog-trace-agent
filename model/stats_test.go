package model

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const defaultEnv = "default"

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

	srb := NewStatsRawBucket(0, 1e9)

	// No custom aggregators only the defaults
	aggr := []string{}
	for _, s := range testSpans {
		srb.HandleSpan(s, defaultEnv, aggr, nil)
	}
	sb := srb.Export()

	expectedCounts := map[string]int64{
		"A.foo|duration|env:default,resource:α,service:A":     1,
		"A.foo|duration|env:default,resource:β,service:A":     2,
		"B.foo|duration|env:default,resource:γ,service:B":     3,
		"B.foo|duration|env:default,resource:ε,service:B":     4,
		"B.foo|duration|env:default,resource:ζ,service:B":     5,
		"sql.query|duration|env:default,resource:ζ,service:B": 6,
		"sql.query|duration|env:default,resource:δ,service:C": 15,
		"A.foo|errors|env:default,resource:α,service:A":       0,
		"A.foo|errors|env:default,resource:β,service:A":       1,
		"B.foo|errors|env:default,resource:γ,service:B":       0,
		"B.foo|errors|env:default,resource:ε,service:B":       1,
		"B.foo|errors|env:default,resource:ζ,service:B":       0,
		"sql.query|errors|env:default,resource:ζ,service:B":   0,
		"sql.query|errors|env:default,resource:δ,service:C":   0,
		"A.foo|hits|env:default,resource:α,service:A":         1,
		"A.foo|hits|env:default,resource:β,service:A":         1,
		"B.foo|hits|env:default,resource:γ,service:B":         1,
		"B.foo|hits|env:default,resource:ε,service:B":         1,
		"B.foo|hits|env:default,resource:ζ,service:B":         1,
		"sql.query|hits|env:default,resource:ζ,service:B":     1,
		"sql.query|hits|env:default,resource:δ,service:C":     2,
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

	srb := NewStatsRawBucket(0, 1e9)

	// one custom aggregator
	aggr := []string{"version"}
	for _, s := range testSpans {
		srb.HandleSpan(s, defaultEnv, aggr, nil)
	}
	sb := srb.Export()

	expectedCounts := map[string]int64{
		"A.foo|duration|env:default,resource:α,service:A":                 1,
		"A.foo|duration|env:default,resource:β,service:A":                 2,
		"B.foo|duration|env:default,resource:γ,service:B":                 3,
		"B.foo|duration|env:default,resource:ε,service:B":                 4,
		"sql.query|duration|env:default,resource:δ,service:C":             15,
		"A.foo|errors|env:default,resource:α,service:A":                   0,
		"A.foo|errors|env:default,resource:β,service:A":                   1,
		"B.foo|errors|env:default,resource:γ,service:B":                   0,
		"B.foo|errors|env:default,resource:ε,service:B":                   1,
		"sql.query|errors|env:default,resource:δ,service:C":               0,
		"A.foo|hits|env:default,resource:α,service:A":                     1,
		"A.foo|hits|env:default,resource:β,service:A":                     1,
		"B.foo|hits|env:default,resource:γ,service:B":                     1,
		"B.foo|hits|env:default,resource:ε,service:B":                     1,
		"sql.query|hits|env:default,resource:δ,service:C":                 2,
		"sql.query|errors|env:default,resource:ζ,service:B,version:1.4":   0,
		"sql.query|hits|env:default,resource:ζ,service:B,version:1.4":     1,
		"sql.query|duration|env:default,resource:ζ,service:B,version:1.4": 6,
		"B.foo|errors|env:default,resource:ζ,service:B,version:1.3":       0,
		"B.foo|duration|env:default,resource:ζ,service:B,version:1.3":     5,
		"B.foo|hits|env:default,resource:ζ,service:B,version:1.3":         1,
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

func TestStatsBucketMany(t *testing.T) {
	if testing.Short() {
		return
	}

	assert := assert.New(t)

	templateSpan := Span{Service: "A", Name: "A.foo", Resource: "α", Duration: 7}
	const n = 100000

	srb := NewStatsRawBucket(0, 1e9)

	// No custom aggregators only the defaults
	aggr := []string{}
	for i := 0; i < n; i++ {
		s := templateSpan
		s.Resource = "α" + strconv.Itoa(i)
		srbCopy := *srb
		srbCopy.HandleSpan(s, defaultEnv, aggr, nil)
	}
	sb := srb.Export()

	assert.Len(sb.Counts, 3*n, "Missing counts %d != %d", len(sb.Counts), 3*n)
	for ckey, c := range sb.Counts {
		if strings.Contains(ckey, "|duration|") {
			assert.Equal(7, c.Value, "duration %s wrong value", ckey)
		}
		if strings.Contains(ckey, "|errors|") {
			assert.Equal(0, c.Value, "errors %s wrong value", ckey)
		}
		if strings.Contains(ckey, "|hits|") {
			assert.Equal(1, c.Value, "hits %s wrong value", ckey)
		}
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

	srb := NewStatsRawBucket(0, 1e9)
	aggr := []string{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, s := range testSpans {
			srb.HandleSpan(s, defaultEnv, aggr, nil)
		}
	}
}

// it's important to have these defined as var and not const/inline
// else compiler performs compile-time optimization when using + with strings
var grainName = "mysql.query"
var grainMeasure = "duration"
var grainAggr = "resource:SELECT * FROM stuff,service:mysql"

// testing out various way of doing string ops, to check which one is most efficient
func BenchmarkGrainKey(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GrainKey(grainName, grainMeasure, grainAggr)
	}
}

func BenchmarkStringPlus(b *testing.B) {
	if testing.Short() {
		return
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = grainName + "|" + grainMeasure + "|" + grainAggr
	}
}

func BenchmarkSprintf(b *testing.B) {
	if testing.Short() {
		return
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("%s|%s|%s", grainName, grainMeasure, grainAggr)
	}
}

func BenchmarkBufferWriteByte(b *testing.B) {
	if testing.Short() {
		return
	}
	var buf bytes.Buffer
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		buf.WriteString(grainName)
		buf.WriteByte('|')
		buf.WriteString(grainMeasure)
		buf.WriteByte('|')
		buf.WriteString(grainAggr)
		_ = buf.String()
	}
}

func BenchmarkBufferWriteRune(b *testing.B) {
	if testing.Short() {
		return
	}
	var buf bytes.Buffer
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		buf.WriteString(grainName)
		buf.WriteRune('|')
		buf.WriteString(grainMeasure)
		buf.WriteRune('|')
		buf.WriteString(grainAggr)
		_ = buf.String()
	}
}

func BenchmarkStringsJoin(b *testing.B) {
	if testing.Short() {
		return
	}
	a := []string{grainName, grainMeasure, grainAggr}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = strings.Join(a, "|")
	}
}

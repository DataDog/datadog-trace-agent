package model

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/quantile"
	"github.com/stretchr/testify/assert"
)

const defaultEnv = "default"

func testSpans() []Span {
	return []Span{
		Span{Service: "A", Name: "A.foo", Resource: "α", Duration: 1},
		Span{Service: "A", Name: "A.foo", Resource: "β", Duration: 2, Error: 1},
		Span{Service: "B", Name: "B.foo", Resource: "γ", Duration: 3},
		Span{Service: "B", Name: "B.foo", Resource: "ε", Duration: 4, Error: 404},
		Span{Service: "B", Name: "B.foo", Resource: "ζ", Duration: 5, Meta: map[string]string{"version": "1.3"}},
		Span{Service: "B", Name: "sql.query", Resource: "ζ", Duration: 6, Meta: map[string]string{"version": "1.4"}},
		Span{Service: "C", Name: "sql.query", Resource: "δ", Duration: 7},
		Span{Service: "C", Name: "sql.query", Resource: "δ", Duration: 8},
	}
}

func testTrace() Trace {
	// Data below represents a trace with some sublayers, so that we make sure,
	// those data are correctly calculated when aggregating in HandleSpan()
	// A |---------------------------------------------------------------| duration: 100
	// B   |----------------------|                                        duration: 20
	// C     |-----| |---|                                                 duration: 5+3
	return Trace{
		Span{TraceID: 42, SpanID: 42, ParentID: 0, Service: "A",
			Name: "A.foo", Type: "web", Resource: "α", Start: 0, Duration: 100,
			Metrics: map[string]float64{SpanSampleRateMetricKey: 0.5}},
		Span{TraceID: 42, SpanID: 100, ParentID: 42, Service: "B",
			Name: "B.bar", Type: "web", Resource: "α", Start: 1, Duration: 20},
		Span{TraceID: 42, SpanID: 2000, ParentID: 100, Service: "C",
			Name: "sql.query", Type: "sql", Resource: "SELECT value FROM table",
			Start: 2, Duration: 5},
		Span{TraceID: 42, SpanID: 3000, ParentID: 100, Service: "C",
			Name: "sql.query", Type: "sql", Resource: "SELECT ololololo... value FROM table",
			Start: 10, Duration: 3, Error: 1},
	}
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
	for _, s := range testSpans() {
		srb.HandleSpan(s, defaultEnv, aggr, 1.0, nil)
	}
	sb := srb.Export()

	expectedCounts := map[string]float64{
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

	expectedDistributions := map[string]int{
		"A.foo|duration|env:default,resource:α,service:A":     1,
		"A.foo|duration|env:default,resource:β,service:A":     1,
		"B.foo|duration|env:default,resource:γ,service:B":     1,
		"B.foo|duration|env:default,resource:ε,service:B":     1,
		"B.foo|duration|env:default,resource:ζ,service:B":     1,
		"sql.query|duration|env:default,resource:ζ,service:B": 1,
		"sql.query|duration|env:default,resource:δ,service:C": 2,
	}

	for k, v := range sb.Distributions {
		t.Logf("%v: %v", k, v.Summary.Entries)
	}
	assert.Len(sb.Distributions, len(expectedDistributions), "Missing distributions!")
	for dkey, c := range sb.Distributions {
		val, ok := expectedDistributions[dkey]
		if !ok {
			assert.Fail("Unexpected distribution %s", dkey)
		}
		assert.Equal(val, len(c.Summary.Entries), "Distribution %s wrong value", dkey)
	}
}

func TestStatsBucketExtraAggregators(t *testing.T) {
	assert := assert.New(t)

	srb := NewStatsRawBucket(0, 1e9)

	// one custom aggregator
	aggr := []string{"version"}
	for _, s := range testSpans() {
		srb.HandleSpan(s, defaultEnv, aggr, 1.0, nil)
	}
	sb := srb.Export()

	expectedCounts := map[string]float64{
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
		keyFields := strings.Split(ckey, "|")
		tags := NewTagSetFromString(keyFields[2])
		assert.Equal(tags, c.TagSet, "bad tagset for count %s", ckey)
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
		srbCopy.HandleSpan(s, defaultEnv, aggr, 1.0, nil)
	}
	sb := srb.Export()

	assert.Len(sb.Counts, 3*n, "Missing counts %d != %d", len(sb.Counts), 3*n)
	for ckey, c := range sb.Counts {
		if strings.Contains(ckey, "|duration|") {
			assert.Equal(7.0, c.Value, "duration %s wrong value", ckey)
		}
		if strings.Contains(ckey, "|errors|") {
			assert.Equal(0.0, c.Value, "errors %s wrong value", ckey)
		}
		if strings.Contains(ckey, "|hits|") {
			assert.Equal(1.0, c.Value, "hits %s wrong value", ckey)
		}
	}
}

func TestStatsBucketSublayers(t *testing.T) {
	assert := assert.New(t)

	tr := testTrace()
	sublayers := ComputeSublayers(&tr)
	root := tr.GetRoot()
	SetSublayersOnSpan(root, sublayers)

	assert.NotNil(sublayers)

	srb := NewStatsRawBucket(0, 1e9)

	// No custom aggregators only the defaults
	aggr := []string{}
	for _, s := range tr {
		srb.HandleSpan(s, defaultEnv, aggr, root.Weight(), &sublayers)
	}
	sb := srb.Export()

	expectedCounts := map[string]float64{
		"A.foo|_sublayers.duration.by_service|env:default,resource:α,service:A,sublayer_service:A":                                        80,
		"A.foo|_sublayers.duration.by_service|env:default,resource:α,service:A,sublayer_service:B":                                        12,
		"A.foo|_sublayers.duration.by_service|env:default,resource:α,service:A,sublayer_service:C":                                        8,
		"A.foo|_sublayers.duration.by_type|env:default,resource:α,service:A,sublayer_type:sql":                                            8,
		"A.foo|_sublayers.duration.by_type|env:default,resource:α,service:A,sublayer_type:web":                                            92,
		"A.foo|_sublayers.span_count|env:default,resource:α,service:A,:":                                                                  4,
		"A.foo|duration|env:default,resource:α,service:A":                                                                                 200,
		"A.foo|errors|env:default,resource:α,service:A":                                                                                   0,
		"A.foo|hits|env:default,resource:α,service:A":                                                                                     2,
		"B.bar|_sublayers.duration.by_service|env:default,resource:α,service:B,sublayer_service:A":                                        80,
		"B.bar|_sublayers.duration.by_service|env:default,resource:α,service:B,sublayer_service:B":                                        12,
		"B.bar|_sublayers.duration.by_service|env:default,resource:α,service:B,sublayer_service:C":                                        8,
		"B.bar|_sublayers.duration.by_type|env:default,resource:α,service:B,sublayer_type:sql":                                            8,
		"B.bar|_sublayers.duration.by_type|env:default,resource:α,service:B,sublayer_type:web":                                            92,
		"B.bar|_sublayers.span_count|env:default,resource:α,service:B,:":                                                                  4,
		"B.bar|duration|env:default,resource:α,service:B":                                                                                 40,
		"B.bar|errors|env:default,resource:α,service:B":                                                                                   0,
		"B.bar|hits|env:default,resource:α,service:B":                                                                                     2,
		"sql.query|_sublayers.duration.by_service|env:default,resource:SELECT ololololo... value FROM table,service:C,sublayer_service:A": 80,
		"sql.query|_sublayers.duration.by_service|env:default,resource:SELECT ololololo... value FROM table,service:C,sublayer_service:B": 12,
		"sql.query|_sublayers.duration.by_service|env:default,resource:SELECT ololololo... value FROM table,service:C,sublayer_service:C": 8,
		"sql.query|_sublayers.duration.by_service|env:default,resource:SELECT value FROM table,service:C,sublayer_service:A":              80,
		"sql.query|_sublayers.duration.by_service|env:default,resource:SELECT value FROM table,service:C,sublayer_service:B":              12,
		"sql.query|_sublayers.duration.by_service|env:default,resource:SELECT value FROM table,service:C,sublayer_service:C":              8,
		"sql.query|_sublayers.duration.by_type|env:default,resource:SELECT ololololo... value FROM table,service:C,sublayer_type:sql":     8,
		"sql.query|_sublayers.duration.by_type|env:default,resource:SELECT ololololo... value FROM table,service:C,sublayer_type:web":     92,
		"sql.query|_sublayers.duration.by_type|env:default,resource:SELECT value FROM table,service:C,sublayer_type:sql":                  8,
		"sql.query|_sublayers.duration.by_type|env:default,resource:SELECT value FROM table,service:C,sublayer_type:web":                  92,
		"sql.query|_sublayers.span_count|env:default,resource:SELECT ololololo... value FROM table,service:C,:":                           4,
		"sql.query|_sublayers.span_count|env:default,resource:SELECT value FROM table,service:C,:":                                        4,
		"sql.query|duration|env:default,resource:SELECT ololololo... value FROM table,service:C":                                          6,
		"sql.query|duration|env:default,resource:SELECT value FROM table,service:C":                                                       10,
		"sql.query|errors|env:default,resource:SELECT ololololo... value FROM table,service:C":                                            2,
		"sql.query|errors|env:default,resource:SELECT value FROM table,service:C":                                                         0,
		"sql.query|hits|env:default,resource:SELECT ololololo... value FROM table,service:C":                                              2,
		"sql.query|hits|env:default,resource:SELECT value FROM table,service:C":                                                           2,
	}

	assert.Len(sb.Counts, len(expectedCounts), "Missing counts!")
	for ckey, c := range sb.Counts {
		val, ok := expectedCounts[ckey]
		if !ok {
			assert.Fail("Unexpected count %s", ckey)
		}
		assert.Equal(val, c.Value, "Count %s wrong value", ckey)
		keyFields := strings.Split(ckey, "|")
		tags := NewTagSetFromString(keyFields[2])
		assert.Equal(tags, c.TagSet, "bad tagset for count %s", ckey)
	}

	expectedDistributions := map[string][]quantile.Entry{
		"A.foo|duration|env:default,resource:α,service:A":                                        []quantile.Entry{quantile.Entry{V: 100, G: 1, Delta: 0}},
		"B.bar|duration|env:default,resource:α,service:B":                                        []quantile.Entry{quantile.Entry{V: 20, G: 1, Delta: 0}},
		"sql.query|duration|env:default,resource:SELECT value FROM table,service:C":              []quantile.Entry{quantile.Entry{V: 5, G: 1, Delta: 0}},
		"sql.query|duration|env:default,resource:SELECT ololololo... value FROM table,service:C": []quantile.Entry{quantile.Entry{V: 3, G: 1, Delta: 0}},
	}

	assert.Len(sb.Distributions, len(expectedDistributions), "Missing distributions!")
	for dkey, d := range sb.Distributions {
		val, ok := expectedDistributions[dkey]
		if !ok {
			assert.Fail("Unexpected distribution %s", dkey)
		}
		assert.Equal(val, d.Summary.Entries, "Distribution %s wrong value", dkey)
		keyFields := strings.Split(dkey, "|")
		tags := NewTagSetFromString(keyFields[2])
		assert.Equal(tags, d.TagSet, "bad tagset for distribution %s", dkey)
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
		for _, s := range testSpans() {
			srb.HandleSpan(s, defaultEnv, aggr, 1.0, nil)
		}
	}
}

func BenchmarkHandleSpanSublayers(b *testing.B) {

	srb := NewStatsRawBucket(0, 1e9)
	aggr := []string{}

	tr := testTrace()
	sublayers := ComputeSublayers(&tr)
	root := tr.GetRoot()
	SetSublayersOnSpan(root, sublayers)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, s := range tr {
			srb.HandleSpan(s, defaultEnv, aggr, root.Weight(), &sublayers)
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

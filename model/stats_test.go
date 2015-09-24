package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounts(t *testing.T) {
	assert := assert.New(t)

	tags := NewTagsFromString("version:34.42,resource:/dash/list,service:dogweb")
	testCounts := []Count{
		NewCount(HITS, tags),
		NewCount(ERRORS, tags),
		NewCount(DURATION, tags),
		NewCount("custom_size", tags),
	}

	// Our fake spans
	spans := []Span{
		Span{Duration: 10},
		Span{Error: 1, Duration: 25},
		Span{
			Duration: 15,
			Metrics:  map[string]int64{"custom_size": 3},
		},
		Span{
			Duration: 0,
			Metrics:  map[string]int64{"custom_size": 10},
		},
	}

	// add spans
	var err error
	for i := range testCounts {
		for _, s := range spans {
			c := testCounts[i]
			testCounts[i], err = c.Add(s)
			if c.Name == "custom_size" && s.Metrics == nil {
				assert.NotNil(err)
			} else {
				assert.Nil(err)
			}
		}
	}

	for _, c := range testCounts {
		switch c.Name {
		case HITS:
			assert.Equal(4, int(c.Value))
		case ERRORS:
			assert.Equal(1, int(c.Value))
		case DURATION:
			assert.Equal(50, int(c.Value))
		case "custom_size":
			assert.Equal(13, int(c.Value))
		default:
			t.Fatalf("Was not supposed to handle count %v", c)
		}
	}
}

var testSpans = []Span{
	Span{Service: "A", Resource: "α", Duration: 1},
	Span{Service: "A", Resource: "β", Duration: 2, Error: 1},
	Span{Service: "B", Resource: "γ", Duration: 3},
	Span{Service: "B", Resource: "ε", Duration: 4, Error: 404},
	Span{Service: "B", Resource: "ζ", Duration: 5, Meta: map[string]string{"version": "1.3"}},
	Span{Service: "B", Resource: "ζ", Duration: 6, Meta: map[string]string{"version": "1.4"}},
	Span{Service: "C", Resource: "δ", Duration: 7},
	Span{Service: "C", Resource: "δ", Duration: 8},
}

func TestStatsBucketDefault(t *testing.T) {
	assert := assert.New(t)

	sb := NewStatsBucket(0, 1e9)
	aggr := []string{"service", "resource"}
	for _, s := range testSpans {
		sb.HandleSpan(s, aggr)
	}

	expectedCounts := map[string]int64{
		"hits|resource:α,service:A":     1,
		"hits|resource:β,service:A":     1,
		"hits|resource:γ,service:B":     1,
		"hits|resource:ε,service:B":     1,
		"hits|resource:ζ,service:B":     2,
		"hits|resource:δ,service:C":     2,
		"errors|resource:α,service:A":   0,
		"errors|resource:β,service:A":   1,
		"errors|resource:γ,service:B":   0,
		"errors|resource:ε,service:B":   1,
		"errors|resource:ζ,service:B":   0,
		"errors|resource:δ,service:C":   0,
		"duration|resource:α,service:A": 1,
		"duration|resource:β,service:A": 2,
		"duration|resource:γ,service:B": 3,
		"duration|resource:ε,service:B": 4,
		"duration|resource:ζ,service:B": 11,
		"duration|resource:δ,service:C": 15,
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
	aggr := []string{"service", "resource", "version"}
	for _, s := range testSpans {
		sb.HandleSpan(s, aggr)
	}

	expectedCounts := map[string]int64{
		"hits|resource:α,service:A":                 1,
		"hits|resource:β,service:A":                 1,
		"hits|resource:γ,service:B":                 1,
		"hits|resource:ε,service:B":                 1,
		"hits|resource:ζ,service:B,version:1.3":     1,
		"hits|resource:ζ,service:B,version:1.4":     1,
		"hits|resource:δ,service:C":                 2,
		"errors|resource:α,service:A":               0,
		"errors|resource:β,service:A":               1,
		"errors|resource:γ,service:B":               0,
		"errors|resource:ε,service:B":               1,
		"errors|resource:ζ,service:B,version:1.3":   0,
		"errors|resource:ζ,service:B,version:1.4":   0,
		"errors|resource:δ,service:C":               0,
		"duration|resource:α,service:A":             1,
		"duration|resource:β,service:A":             2,
		"duration|resource:γ,service:B":             3,
		"duration|resource:ε,service:B":             4,
		"duration|resource:ζ,service:B,version:1.3": 5,
		"duration|resource:ζ,service:B,version:1.4": 6,
		"duration|resource:δ,service:C":             15,
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

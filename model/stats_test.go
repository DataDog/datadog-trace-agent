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
	Span{Layer: "A", Resource: "α", Duration: 1},
	Span{Layer: "A", Resource: "β", Duration: 2, Error: 1},
	Span{Layer: "B.a", Resource: "γ", Duration: 3},
	Span{Layer: "B.a", Resource: "ε", Duration: 4, Error: 404},
	Span{Layer: "B.b", Resource: "ζ", Duration: 5, Meta: map[string]string{"version": "1.3"}},
	Span{Layer: "B", Resource: "ζ", Duration: 6, Meta: map[string]string{"version": "1.4"}},
	Span{Layer: "C", Resource: "δ", Duration: 7},
	Span{Layer: "C", Resource: "δ", Duration: 8},
}

func TestStatsBucketDefault(t *testing.T) {
	assert := assert.New(t)

	sb := NewStatsBucket(0, 1e9)
	aggr := []string{"app", "layer", "resource"}
	for _, s := range testSpans {
		sb.HandleSpan(s, aggr)
	}

	expectedCounts := map[string]int64{
		"duration|app:A,resource:α":         1,
		"duration|app:A,resource:β":         2,
		"duration|app:B,layer:a,resource:γ": 3,
		"duration|app:B,layer:a,resource:ε": 4,
		"duration|app:B,layer:b,resource:ζ": 5,
		"duration|app:B,resource:ζ":         6,
		"duration|app:C,resource:δ":         15,
		"errors|app:A,resource:α":           0,
		"errors|app:A,resource:β":           1,
		"errors|app:B,layer:a,resource:γ":   0,
		"errors|app:B,layer:a,resource:ε":   1,
		"errors|app:B,layer:b,resource:ζ":   0,
		"errors|app:B,resource:ζ":           0,
		"errors|app:C,resource:δ":           0,
		"hits|app:A,resource:α":             1,
		"hits|app:A,resource:β":             1,
		"hits|app:B,layer:a,resource:γ":     1,
		"hits|app:B,layer:a,resource:ε":     1,
		"hits|app:B,layer:b,resource:ζ":     1,
		"hits|app:B,resource:ζ":             1,
		"hits|app:C,resource:δ":             2,
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
	aggr := []string{"layer", "version"}
	for _, s := range testSpans {
		sb.HandleSpan(s, aggr)
	}

	expectedCounts := map[string]int64{
		"duration|app:A":                     3,
		"duration|app:B,layer:a":             7,
		"duration|app:B,layer:b,version:1.3": 5,
		"duration|app:B,version:1.4":         6,
		"duration|app:C":                     15,
		"errors|app:A":                       1,
		"errors|app:B,layer:a":               1,
		"errors|app:B,layer:b,version:1.3":   0,
		"errors|app:B,version:1.4":           0,
		"errors|app:C":                       0,
		"hits|app:A":                         2,
		"hits|app:B,layer:a":                 2,
		"hits|app:B,layer:b,version:1.3":     1,
		"hits|app:B,version:1.4":             1,
		"hits|app:C":                         2,
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

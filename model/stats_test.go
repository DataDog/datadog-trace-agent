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
			assert.Equal(4, c.Value)
		case ERRORS:
			assert.Equal(1, c.Value)
		case DURATION:
			assert.Equal(50, c.Value)
		case "custom_size":
			assert.Equal(13, c.Value)
		default:
			t.Fatalf("Was not supposed to handle count %v", c)
		}
	}
}

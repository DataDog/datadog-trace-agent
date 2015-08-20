package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Hardcoded metric names for ease of reference
const (
	HITS     string = "hits"
	ERRORS          = "errors"
	DURATION        = "duration"
)

// These represents the default stats we keep track of
var DefaultMetrics = [3]string{HITS, ERRORS, DURATION}

// Count represents one specific "metric" we track for a given tag set, it accumulates new values, optionally keeping track of the distribution
type Count struct {
	Name         string       `json:"name"`         // represents the entity we count, e.g. "hits", "errors", "time"
	Tags         []Tag        `json:"tags"`         // list of dimensions for which we account this Count
	Value        int64        `json:"value"`        // accumulated values
	Distribution Distribution `json:"distribution"` // optional, represents distribution of values and refs of traces for the spectrum of values
}

// NewCount returns a new Count for a resource, with a given set of tags and an epsilon precision
func NewCount(m string, tags *[]Tag, eps float64) *Count {
	// FIXME: how to handle tracking the distribution of other than DefaultMetrics?
	var d Distribution
	if m == DURATION {
		d = NewDistribution(eps)
	}
	return &Count{
		Name:         m,
		Tags:         *tags,
		Value:        0,
		Distribution: d,
	}
}

// Add adds a Span to a Count
func (c *Count) Add(s *Span) bool {
	switch c.Name {
	case HITS:
		c.Value++
		return true
	case ERRORS:
		c.Value++
		return true // always keep error traces? probably stupid if errors are not aggregated in some way
	case DURATION:
		c.Value += int64(s.Duration * 1e9)
		keep := c.Distribution.Insert(s.Duration, s.SpanID)
		return keep
	default:
		panic(fmt.Errorf(fmt.Sprintf("Don't know how to handle a '%s' count", c.Name)))
	}
}

// StatsBucket is a time bucket to track statistic around multiple Counts
type StatsBucket struct {
	Start    float64
	Duration float64
	Counts   map[string]*Count
	//	Quantiles map[string]*Quantiles
	Eps float64
}

// MarshalJSON returns a JSON representation
func (sb *StatsBucket) MarshalJSON() ([]byte, error) {
	// FIXME: add quantiles
	flatCounts := make([]*Count, len(sb.Counts))
	i := 0
	for _, val := range sb.Counts {
		flatCounts[i] = val
		i++
	}
	return json.Marshal(map[string]interface{}{
		"start":    sb.Start,
		"duration": sb.Duration,
		"counts":   flatCounts,
	})
}

// CountKey returns to name of the key counting a specific resource/tag
func CountKey(m string, tags []Tag) string {
	s := make([]string, len(tags))
	for i, t := range tags {
		s[i] = t.String()
	}
	sort.Strings(s)
	return fmt.Sprintf("metric:%s|tags:%s", m, strings.Join(s, ","))
}

// NewStatsBucket opens a new bucket at this time and initializes it properly
func NewStatsBucket(eps float64) *StatsBucket {
	m := make(map[string]*Count)
	return &StatsBucket{
		Eps:    eps,
		Start:  Now(),
		Counts: m,
	}
}

// HandleSpan adds the span to this bucket stats
func (sb *StatsBucket) HandleSpan(s *Span) bool {
	keep := false

	// FIXME: clean and implement generic way of generating tag sets and metric names

	// by service
	sTag := Tag{Name: "service", Value: s.Service}
	byS := []Tag{sTag}
	if sb.addInDimension(s, &byS) {
		keep = true
	}

	// by (service, resource)
	rTag := Tag{Name: "resource", Value: s.Resource}
	bySR := []Tag{sTag, rTag}
	if sb.addInDimension(s, &bySR) {
		keep = true
	}

	return keep
}

func (sb *StatsBucket) addInDimension(s *Span, tags *[]Tag) bool {
	// FIXME: here add the ability to add more than the DefaultMetrics?
	keep := false
	for _, m := range DefaultMetrics {
		if sb.addToCount(m, s, tags) {
			keep = true
		}
	}

	return keep
}

func (sb *StatsBucket) addToCount(metric string, s *Span, tags *[]Tag) bool {
	ckey := CountKey(metric, *tags)

	_, ok := sb.Counts[ckey]
	if !ok {
		sb.Counts[ckey] = NewCount(metric, tags, sb.Eps)
	}

	return sb.Counts[ckey].Add(s)
}

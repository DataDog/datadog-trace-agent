package model

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Hardcoded metric names for ease of reference
const (
	HITS     string = "hits"
	ERRORS          = "errors"
	DURATION        = "duration"
)

// These represents the default stats we keep track of, Counts
var DefaultCounts = [...]string{HITS, ERRORS, DURATION}

// and Distributions
var DefaultDistributions = [...]string{DURATION}

// Count represents one specific "metric" we track for a given dimension
type Count struct {
	Name   string `json:"name"`   // represents the entity we count, e.g. "hits", "errors", "time"
	TagSet TagSet `json:"tagset"` // set of tags for which we account this Distribution
	Value  int64  `json:"value"`  // accumulated values
}

// Distribution represents a true image of the spectrum of values, allowing arbitrary quantile queries
type Distribution struct {
	Name    string  `json:"name"`    // represents the entity we count, e.g. "hits", "errors", "time"
	TagSet  TagSet  `json:"tagset"`  // set of tags for which we account this Distribution
	Summary Summary `json:"summary"` // actual representation of data
}

// NewCount returns a new Count for a metric and a given tag set
func NewCount(m string, tgs *TagSet) *Count {
	return &Count{
		Name:   m,
		TagSet: *tgs,
		Value:  0,
	}
}

// Add adds a Span to a Count
func (c *Count) Add(s *Span) {
	switch c.Name {
	case HITS, ERRORS:
		c.Value++
	case DURATION:
		c.Value += s.Duration
	default:
		// arbitrary metrics implementation
		val, ok := s.Metrics[c.Name]
		if !ok {
			panic(fmt.Errorf(fmt.Sprintf("Don't know how to handle a '%s' count", c.Name)))
		}
		c.Value += val
	}
}

// NewDistribution returns a new Distribution for a metric and a given tag set
func NewDistribution(m string, tgs *TagSet, eps float64) *Distribution {
	return &Distribution{
		Name:    m,
		TagSet:  *tgs,
		Summary: NewSummary(eps),
	}
}

// Add inserts the proper values in a given distribution from a span
func (d *Distribution) Add(s *Span) {
	if d.Name == DURATION {
		d.Summary.Insert(s.Duration, s.SpanID)
	} else {
		val, ok := s.Metrics[d.Name]
		if !ok {
			panic(fmt.Errorf("Don't know how to handle a '%s' distribution", d.Name))
		}
		d.Summary.Insert(val, s.SpanID)
	}
}

// StatsBucket is a time bucket to track statistic around multiple Counts
type StatsBucket struct {
	Start    int64
	Duration int64
	Eps      float64

	// stats indexed by keys
	Counts        map[string]*Count
	Distributions map[string]*Distribution
}

// NewStatsBucket opens a new bucket at this time and initializes it properly
func NewStatsBucket(eps float64) *StatsBucket {
	counts := make(map[string]*Count)
	distros := make(map[string]*Distribution)

	// The only non-initialized value is the Duration which should be set by whoever closes that bucket
	return &StatsBucket{
		Start:         Now(),
		Eps:           eps,
		Counts:        counts,
		Distributions: distros,
	}
}

// MarshalJSON returns a JSON representation of a bucket, flattening stats
func (sb *StatsBucket) MarshalJSON() ([]byte, error) {
	if sb.Duration == 0 {
		panic(errors.New("Trying to marshal a bucket that has not been closed"))
	}

	flatCounts := make([]*Count, len(sb.Counts))
	i := 0
	for _, val := range sb.Counts {
		flatCounts[i] = val
		i++
	}
	flatDistros := make([]*Distribution, len(sb.Distributions))
	i = 0
	for _, val := range sb.Distributions {
		flatDistros[i] = val
		i++
	}
	return json.Marshal(map[string]interface{}{
		"start":         sb.Start,
		"duration":      sb.Duration,
		"epsilon":       sb.Eps,
		"counts":        flatCounts,
		"distributions": flatDistros,
	})
}

// HandleSpan adds the span to this bucket stats
func (sb *StatsBucket) HandleSpan(s *Span) {
	// by service
	sTag := Tag{Name: "service", Value: s.Service}
	byS := TagSet{sTag}
	sb.addToTagSet(s, &byS)

	// by (service, resource)
	rTag := Tag{Name: "resource", Value: s.Resource}
	bySR := TagSet{sTag, rTag}
	sb.addToTagSet(s, &bySR)

	// TODO by (service) or (service, resource) union preset tags in the config (from s.Metadata)
}

func (sb *StatsBucket) addToTagSet(s *Span, tgs *TagSet) {
	for _, m := range DefaultCounts {
		sb.addToCount(m, s, tgs)
	}
	// TODO add for s.Metrics ability to define arbitrary counts and distros

	for _, m := range DefaultDistributions {
		sb.addToDistribution(m, s, tgs)
	}
}

func (sb *StatsBucket) addToCount(m string, s *Span, tgs *TagSet) {
	ckey := tgs.GetTagKey(m)

	if _, ok := sb.Counts[ckey]; !ok {
		sb.Counts[ckey] = NewCount(m, tgs)
	}

	sb.Counts[ckey].Add(s)
}

func (sb *StatsBucket) addToDistribution(m string, s *Span, tgs *TagSet) {
	ckey := tgs.GetTagKey(m)

	if _, ok := sb.Distributions[ckey]; !ok {
		sb.Distributions[ckey] = NewDistribution(m, tgs, sb.Eps)
	}

	sb.Distributions[ckey].Add(s)
}

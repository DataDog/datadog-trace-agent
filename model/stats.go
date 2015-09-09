package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/DataDog/raclette/quantile"
	log "github.com/cihub/seelog"
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

// Count represents one specific "metric" we track for a given tagset
type Count struct {
	Key    string `json:"key"`
	Name   string `json:"name"`   // represents the entity we count, e.g. "hits", "errors", "time"
	TagSet TagSet `json:"tagset"` // set of tags for which we account this Distribution
	Value  int64  `json:"value"`  // accumulated values
}

// Distribution represents a true image of the spectrum of values, allowing arbitrary quantile queries
type Distribution struct {
	Key     string            `json:"key"`
	Name    string            `json:"name"`    // represents the entity we count, e.g. "hits", "errors", "time"
	TagSet  TagSet            `json:"tagset"`  // set of tags for which we account this Distribution
	Summary *quantile.Summary `json:"summary"` // actual representation of data
}

// NewCount returns a new Count for a metric and a given tag set
func NewCount(m string, tgs TagSet) Count {
	c := Count{Name: m, TagSet: tgs, Value: 0}
	tagStrings := make([]string, len(c.TagSet))
	for i, t := range c.TagSet {
		tagStrings[i] = fmt.Sprintf("%s:%s", t.Name, t.Value)
	}
	sort.Strings(tagStrings)
	c.Key = fmt.Sprintf("Count(name=%s,tags=%s)", c.Name, strings.Join(tagStrings, ","))
	return c
}

// Add adds a Span to a Count, returns an error if it cannot add values
func (c Count) Add(s Span) (Count, error) {
	newc := Count{
		Key:    c.Key,
		Name:   c.Name,
		TagSet: c.TagSet,
	}

	switch c.Name {
	case HITS:
		newc.Value = c.Value + 1
	case ERRORS:
		if s.Error != 0 {
			newc.Value = c.Value + 1
		} else {
			return c, nil
		}
	case DURATION:
		newc.Value = c.Value + s.Duration
	default:
		// arbitrary metrics implementation
		if s.Metrics != nil {
			val, ok := s.Metrics[c.Name]
			if !ok {
				return c, fmt.Errorf("Count %s was not initialized", c.Name)
			}
			newc.Value = c.Value + val
		} else {
			return c, fmt.Errorf("Not adding span metrics %v to count %s, not compatible", s.Metrics, c.Name)
		}
	}
	return newc, nil
}

// NewDistribution returns a new Distribution for a metric and a given tag set
func NewDistribution(m string, tgs TagSet) Distribution {
	return Distribution{
		Name:    m,
		TagSet:  tgs,
		Summary: quantile.NewSummary(),
	}
}

// Add inserts the proper values in a given distribution from a span
func (d Distribution) Add(s Span) {
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
	Start    int64 // timestamp of start in our format
	Duration int64 // duration of a bucket in nanoseconds

	// stats indexed by keys
	Counts        map[string]Count        // All the true counts we keep
	Distributions map[string]Distribution // All the true distribution we keep to answer quantile queries
}

// NewStatsBucket opens a new bucket for time ts and initializes it properly
func NewStatsBucket(ts int64) StatsBucket {
	counts := make(map[string]Count)
	distros := make(map[string]Distribution)

	// The only non-initialized value is the Duration which should be set by whoever closes that bucket
	return StatsBucket{
		Start:         ts,
		Counts:        counts,
		Distributions: distros,
	}
}

// MarshalJSON returns a JSON representation of a bucket, flattening stats
func (sb StatsBucket) MarshalJSON() ([]byte, error) {
	if sb.Duration == 0 {
		panic(errors.New("Trying to marshal a bucket that has not been closed"))
	}

	flatCounts := make([]Count, len(sb.Counts))
	i := 0
	for _, val := range sb.Counts {
		flatCounts[i] = val
		i++
	}
	flatDistros := make([]Distribution, len(sb.Distributions))
	i = 0
	for _, val := range sb.Distributions {
		flatDistros[i] = val
		i++
	}
	return json.Marshal(map[string]interface{}{
		"start":         sb.Start,
		"duration":      sb.Duration,
		"counts":        flatCounts,
		"distributions": flatDistros,
	})
}

// HandleSpan adds the span to this bucket stats
func (sb *StatsBucket) HandleSpan(s Span) {
	// by service
	sTag := Tag{Name: "service", Value: s.Service}
	byS := TagSet{sTag}
	sb.addToTagSet(s, byS)

	// by (service, resource)
	rTag := Tag{Name: "resource", Value: s.Resource}
	bySR := TagSet{sTag, rTag}
	sb.addToTagSet(s, bySR)

	// TODO by (service) or (service, resource) union preset tags in the config (from s.Metadata)
}

func (sb StatsBucket) addToTagSet(s Span, tgs TagSet) {
	for _, m := range DefaultCounts {
		sb.addToCount(m, s, tgs)
	}

	// TODO add for s.Metrics ability to define arbitrary counts and distros, check some config?

	for _, m := range DefaultDistributions {
		sb.addToDistribution(m, s, tgs)
	}
}

func (sb StatsBucket) addToCount(m string, s Span, tgs TagSet) {
	ckey := tgs.TagKey(m)

	if _, ok := sb.Counts[ckey]; !ok {
		sb.Counts[ckey] = NewCount(m, tgs)
	}

	var err error
	sb.Counts[ckey], err = sb.Counts[ckey].Add(s)
	if err != nil {
		log.Infof("Not adding span %d to count %s/%s, %s", s.SpanID, m, ckey, err)
	}
}

func (sb StatsBucket) addToDistribution(m string, s Span, tgs TagSet) {
	ckey := tgs.TagKey(m)

	if _, ok := sb.Distributions[ckey]; !ok {
		sb.Distributions[ckey] = NewDistribution(m, tgs)
	}

	sb.Distributions[ckey].Add(s)
}

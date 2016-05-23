package model

import (
	"fmt"
	"math"
	"time"

	"github.com/DataDog/raclette/quantile"
	log "github.com/cihub/seelog"
)

// Hardcoded metric names for ease of reference
const (
	HITS     string = "hits"
	ERRORS          = "errors"
	DURATION        = "duration"
)

var (
	// DefaultCounts is an array of the measures we represent as Count by default
	DefaultCounts = [...]string{HITS, ERRORS, DURATION}
	// DefaultDistributions is an array of the measures we represent as Distribution by default
	// Not really used right now as we don't have a way to easily add new distros
	DefaultDistributions = [...]string{DURATION}
)

// Count represents one specific "metric" we track for a given tagset
type Count struct {
	Key    string `json:"key"`
	Name   string `json:"name"`   // represents the entity we count, e.g. "hits", "errors", "time"
	TagSet TagSet `json:"tagset"` // set of tags for which we account this Distribution

	Value float64 `json:"value"` // accumulated values
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
	return Count{Key: tgs.TagKey(m), Name: m, TagSet: tgs, Value: 0.0}
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
		newc.Value = c.Value + float64(s.Duration)
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

// Merge is used when 2 Counts represent the same thing and adds Values
func (c Count) Merge(c2 Count) Count {
	if c.Key != c2.Key {
		err := fmt.Errorf("Trying to merge non-homogoneous counts [%s] and [%s]", c.Key, c2.Key)
		panic(err)
	}

	return Count{
		Key:    c.Key,
		Name:   c.Name,
		TagSet: c.TagSet,
		Value:  c.Value + c2.Value,
	}
}

// NewDistribution returns a new Distribution for a metric and a given tag set
func NewDistribution(m string, tgs TagSet) Distribution {
	return Distribution{
		Key:     tgs.TagKey(m),
		Name:    m,
		TagSet:  tgs,
		Summary: quantile.NewSummary(),
	}
}

// Add inserts the proper values in a given distribution from a span
func (d Distribution) Add(s Span, res time.Duration) {
	var val float64

	// only use the resolution on our duration distrib
	// which a number of nanoseconds
	if d.Name == DURATION {
		val = NSTimestampToFloat(s.Duration, res)
	} else {
		var ok bool
		// FIXME: s.Metrics should have float64 vals
		var intval int64
		intval, ok = s.Metrics[d.Name]
		if !ok {
			panic(fmt.Errorf("Don't know how to handle a '%s' distribution", d.Name))
		}
		val = float64(intval)
	}

	d.Summary.Insert(val, s.SpanID)
}

// Merge is used when 2 Distributions represent the same thing and it merges the 2 underlying summaries
func (d Distribution) Merge(d2 Distribution) {
	// We don't check tagsets for distributions as we reaggregate without reallocating new structs
	d.Summary.Merge(d2.Summary)
}

// StatsBucket is a time bucket to track statistic around multiple Counts
type StatsBucket struct {
	Start    int64 // timestamp of start in our format
	Duration int64 // duration of a bucket in nanoseconds

	DistroResolution time.Duration // the time res we use for distros

	// stats indexed by keys
	Counts        map[string]Count        // All the true counts we keep
	Distributions map[string]Distribution // All the true distribution we keep to answer quantile queries
}

// NewStatsBucket opens a new bucket for time ts and initializes it properly
func NewStatsBucket(ts, d int64, res time.Duration) StatsBucket {
	counts := make(map[string]Count)
	distros := make(map[string]Distribution)

	// The only non-initialized value is the Duration which should be set by whoever closes that bucket
	return StatsBucket{
		Start:            ts,
		Duration:         d,
		Counts:           counts,
		Distributions:    distros,
		DistroResolution: res,
	}
}

// HandleSpan adds the span to this bucket stats, aggregated with the finest grain matching given aggregators
func (sb *StatsBucket) HandleSpan(s Span, aggregators []string) {
	finestGrain := TagSet{}

	for _, agg := range aggregators {
		switch agg {
		case "service":
			finestGrain = append(finestGrain, Tag{Name: "service", Value: s.Service})
		case "name":
			finestGrain = append(finestGrain, Tag{Name: "name", Value: s.Name})
		case "resource":
			finestGrain = append(finestGrain, Tag{Name: "resource", Value: s.Resource})
		// custom aggregators asked by people
		default:
			val, ok := s.Meta[agg]
			if ok {
				finestGrain = append(finestGrain, Tag{Name: agg, Value: val})
			}
		}
	}

	sb.addToTagSet(s, finestGrain)
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

	sb.Distributions[ckey].Add(s, sb.DistroResolution)
}

// IsEmpty just says if this stats bucket has no information (in which case it's useless)
func (sb StatsBucket) IsEmpty() bool {
	return len(sb.Counts) == 0 && len(sb.Distributions) == 0
}

// NSTimestampToFloat converts a nanosec timestamp into a float nanosecond timestamp truncated at given resoultion
func NSTimestampToFloat(ns int64, res time.Duration) float64 {
	return math.Trunc(float64(ns)/float64(res)) * float64(res)
}

package model

import (
	"fmt"
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
	return Count{Key: tgs.TagKey(m), Name: m, TagSet: tgs, Value: 0}
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

func CopyDistribution(d Distribution) Distribution {
	dcopy := Distribution{
		Key:    d.Key,
		Name:   d.Name,
		TagSet: d.TagSet,
	}
	*dcopy.Summary = *d.Summary
	return dcopy
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

// Merge is used when 2 Distributions represent the same thing and it merges the 2 underlying summaries
func (d Distribution) Merge(d2 Distribution) {
	// We don't check tagsets for distributions as we reaggregate without reallocating new structs
	d.Summary.Merge(d2.Summary)
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
func NewStatsBucket(ts, d int64) StatsBucket {
	counts := make(map[string]Count)
	distros := make(map[string]Distribution)

	// The only non-initialized value is the Duration which should be set by whoever closes that bucket
	return StatsBucket{
		Start:         ts,
		Duration:      d,
		Counts:        counts,
		Distributions: distros,
	}
}

// HandleSpan adds the span to this bucket stats, aggregated with the finest grain matching given aggregators
func (sb *StatsBucket) HandleSpan(s Span, aggregators []string) {
	finestGrain := TagSet{}

	for _, agg := range aggregators {
		switch agg {
		case "service": // backwards-compat with aggregators
		case "layer":
			// peel layers
			layers := strings.Split(s.Layer, ".")
			finestGrain = append(finestGrain, Tag{Name: "app", Value: layers[0]})

			if len(layers) > 1 {
				// right now don't support nested layers
				finestGrain = append(finestGrain, Tag{Name: "layer", Value: layers[1]})
			}
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

	sb.Distributions[ckey].Add(s)
}

// IsEmpty just says if this stats bucket has no information (in which case it's useless)
func (sb StatsBucket) IsEmpty() bool {
	return len(sb.Counts) == 0 && len(sb.Distributions) == 0
}

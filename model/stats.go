package model

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/DataDog/raclette/quantile"
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

// Add adds some values to one count
func (c Count) Add(v float64) Count {
	c.Value += v
	return c
}

// Merge is used when 2 Counts represent the same thing and adds Values
func (c Count) Merge(c2 Count) Count {
	if c.Key != c2.Key {
		err := fmt.Errorf("Trying to merge non-homogoneous counts [%s] and [%s]", c.Key, c2.Key)
		panic(err)
	}

	c.Value += c2.Value
	return c
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
func (d Distribution) Add(v float64, sampleID uint64) {
	d.Summary.Insert(v, sampleID)
}

// Merge is used when 2 Distributions represent the same thing and it merges the 2 underlying summaries
func (d Distribution) Merge(d2 Distribution) {
	// We don't check tagsets for distributions as we reaggregate without reallocating new structs
	d.Summary.Merge(d2.Summary)
}

// Copy returns a distro with the same data but a different underlying summary
func (d Distribution) Copy() Distribution {
	d2 := Distribution(d)
	d2.Summary = d.Summary.Copy()
	return d2
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
			finestGrain = append(finestGrain, Tag{"service", s.Service})
		case "name":
			finestGrain = append(finestGrain, Tag{"name", s.Name})
		case "resource":
			finestGrain = append(finestGrain, Tag{"resource", s.Resource})
		// custom aggregators asked by people
		default:
			val, ok := s.Meta[agg]
			if ok {
				finestGrain = append(finestGrain, Tag{agg, val})
			}
		}
	}

	sb.addToTagSet(s, finestGrain)
}

func (sb StatsBucket) addToTagSet(s Span, tgs TagSet) {
	// HITS
	sb.addToCount(HITS, 1, tgs)
	// FIXME: this does not really make sense actually
	// ERRORS
	if s.Error != 0 {
		sb.addToCount(ERRORS, 1, tgs)
	} else {
		sb.addToCount(ERRORS, 0, tgs)
	}
	// DURATION
	sb.addToCount(DURATION, float64(s.Duration), tgs)

	// TODO add for s.Metrics ability to define arbitrary counts and distros, check some config?
	for m, v := range s.Metrics {
		// produce sublayer statistics, span_count is a special metric used in the UI only
		if strings.HasPrefix(m, "_sublayers") && m != "_sublayers.span_count" {
			// add tags for breaking down sublayers later on
			// skip "_sublayers." then there is "duration.by_service.sublayer_service:XXXX"
			subparsed := strings.SplitN(m[11:], ".", 3)
			if !strings.HasPrefix(subparsed[1], "by_") {
				continue
			}

			sltags := make(TagSet, len(tgs)+1)
			copy(sltags, tgs)
			sltags[len(tgs)] = NewTagFromString(subparsed[2])

			// only extract _sublayers.duration.by_service
			sb.addToCount(m[:len(m)-len(subparsed[2])-1], v, sltags)
		}
	}

	// alter resolution of duration distro
	trundur := nsTimestampToFloat(s.Duration, sb.DistroResolution)
	sb.addToDistribution(DURATION, trundur, s.SpanID, tgs)
}

func (sb StatsBucket) addToCount(m string, v float64, tgs TagSet) {
	ckey := tgs.TagKey(m)

	if _, ok := sb.Counts[ckey]; !ok {
		sb.Counts[ckey] = NewCount(m, tgs)
	}

	sb.Counts[ckey] = sb.Counts[ckey].Add(v)
}

func (sb StatsBucket) addToDistribution(m string, v float64, sampleID uint64, tgs TagSet) {
	ckey := tgs.TagKey(m)

	if _, ok := sb.Distributions[ckey]; !ok {
		sb.Distributions[ckey] = NewDistribution(m, tgs)
	}

	sb.Distributions[ckey].Add(v, sampleID)
}

// IsEmpty just says if this stats bucket has no information (in which case it's useless)
func (sb StatsBucket) IsEmpty() bool {
	return len(sb.Counts) == 0 && len(sb.Distributions) == 0
}

// nsTimestampToFloat converts a nanosec timestamp into a float nanosecond timestamp truncated at given resoultion
func nsTimestampToFloat(ns int64, res time.Duration) float64 {
	return math.Trunc(float64(ns)/float64(res)) * float64(res)
}

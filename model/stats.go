package model

import (
	"fmt"

	"github.com/DataDog/datadog-trace-agent/quantile"
)

// Hardcoded measures names for ease of reference
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
	Key     string `json:"key"`
	Name    string `json:"name"`    // the name of the trace/spans we count (was a member of TagSet)
	Measure string `json:"measure"` // represents the entity we count, e.g. "hits", "errors", "time" (was Name)
	TagSet  TagSet `json:"tagset"`  // set of tags for which we account this Distribution

	TopLevel int64 `json:"top_level"` // number of top-level spans contributing to this count

	Value float64 `json:"value"` // accumulated values
}

// Distribution represents a true image of the spectrum of values, allowing arbitrary quantile queries
type Distribution struct {
	Key     string `json:"key"`
	Name    string `json:"name"`    // the name of the trace/spans we count (was a member of TagSet)
	Measure string `json:"measure"` // represents the entity we count, e.g. "hits", "errors", "time"
	TagSet  TagSet `json:"tagset"`  // set of tags for which we account this Distribution

	TopLevel int64 `json:"top_level"` // number of top-level spans contributing to this count

	Summary *quantile.SliceSummary `json:"summary"` // actual representation of data
}

// GrainKey generates the key used to aggregate counts and distributions
// which is of the form: name|measure|aggr
// for example: serve|duration|service:webserver
func GrainKey(name, measure, aggr string) string {
	return name + "|" + measure + "|" + aggr
}

// NewCount returns a new Count for a metric and a given tag set
func NewCount(m, ckey, name string, tgs TagSet) Count {
	return Count{
		Key:     ckey,
		Name:    name,
		Measure: m,
		TagSet:  tgs, // note: by doing this, tgs is a ref shared by all objects created with the same arg
		Value:   0.0,
	}
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
func NewDistribution(m, ckey, name string, tgs TagSet) Distribution {
	return Distribution{
		Key:     ckey,
		Name:    name,
		Measure: m,
		TagSet:  tgs, // note: by doing this, tgs is a ref shared by all objects created with the same arg
		Summary: quantile.NewSliceSummary(),
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

// Weigh applies a weight factor to a distribution and return the result as a
// new distribution.
func (d Distribution) Weigh(weight float64) Distribution {
	d2 := Distribution(d)
	d2.Summary = quantile.WeighSummary(d.Summary, weight)
	return d2
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

	// stats indexed by keys
	Counts        map[string]Count        // All the true counts we keep
	Distributions map[string]Distribution // All the true distribution we keep to answer quantile queries
}

// NewStatsBucket opens a new bucket for time ts and initializes it properly
func NewStatsBucket(ts, d int64) StatsBucket {
	// The only non-initialized value is the Duration which should be set by whoever closes that bucket
	return StatsBucket{
		Start:         ts,
		Duration:      d,
		Counts:        make(map[string]Count),
		Distributions: make(map[string]Distribution),
	}
}

// IsEmpty just says if this stats bucket has no information (in which case it's useless)
func (sb StatsBucket) IsEmpty() bool {
	return len(sb.Counts) == 0 && len(sb.Distributions) == 0
}

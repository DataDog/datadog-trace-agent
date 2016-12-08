package model

import (
	"bytes"
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

	Value float64 `json:"value"` // accumulated values
}

// Distribution represents a true image of the spectrum of values, allowing arbitrary quantile queries
type Distribution struct {
	Key     string `json:"key"`
	Name    string `json:"name"`    // the name of the trace/spans we count (was a member of TagSet)
	Measure string `json:"measure"` // represents the entity we count, e.g. "hits", "errors", "time"
	TagSet  TagSet `json:"tagset"`  // set of tags for which we account this Distribution

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

	// internal buffer for aggregate strings - not threadsafe
	keyBuf bytes.Buffer
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

func assembleGrain(b *bytes.Buffer, keys, vals []string) (string, TagSet) {
	if len(keys) != len(vals) {
		panic("assembleGrain diff lengths!")
	}

	b.Reset()
	var t TagSet

	for i := range keys {
		b.WriteString(keys[i])
		b.WriteRune(':')
		b.WriteString(vals[i])
		if i != len(keys)-1 {
			b.WriteRune(',')
		}
		t = append(t, Tag{keys[i], vals[i]})
	}

	return b.String(), t
}

// HandleSpan adds the span to this bucket stats, aggregated with the finest grain matching given aggregators
func (sb *StatsBucket) HandleSpan(s Span, env string, aggregators []string, sublayers *[]SublayerValue) {
	if env == "" {
		panic("env should never be empty")
	}

	keys := []string{
		"env",
		"resource",
		"service",
	}
	vals := []string{
		env,
		s.Resource,
		s.Service,
	}

	for _, agg := range aggregators {
		if agg != "resource" && agg != "service" && agg != "env" {
			if v, ok := s.Meta[agg]; ok {
				keys = append(keys, agg)
				vals = append(vals, v)
			}
		}
	}

	grain, tags := assembleGrain(&sb.keyBuf, keys, vals)
	sb.addToTagSet(s, grain, tags)

	// sublayers - special case
	if sublayers != nil {
		for _, sub := range *sublayers {
			subgrain := fmt.Sprintf("%s,%s:%s", grain, sub.Tag.Name, sub.Tag.Value)
			subtags := make(TagSet, len(tags)+1)
			copy(subtags, tags)
			subtags[len(tags)] = sub.Tag

			sb.addToCount(sub.Metric, sub.Value, subgrain, s.Name, subtags)
		}
	}
}

func (sb StatsBucket) addToTagSet(s Span, aggr string, tgs TagSet) {
	// HITS
	sb.addToCount(HITS, 1, aggr, s.Name, tgs)
	// FIXME: this does not really make sense actually
	// ERRORS
	if s.Error != 0 {
		sb.addToCount(ERRORS, 1, aggr, s.Name, tgs)
	} else {
		sb.addToCount(ERRORS, 0, aggr, s.Name, tgs)
	}
	// DURATION
	sb.addToCount(DURATION, float64(s.Duration), aggr, s.Name, tgs)

	// TODO add for s.Metrics ability to define arbitrary counts and distros, check some config?

	// alter resolution of duration distro
	trundur := nsTimestampToFloat(s.Duration)
	sb.addToDistribution(DURATION, trundur, s.SpanID, aggr, s.Name, tgs)
}

func (sb StatsBucket) addToCount(m string, v float64, aggr, name string, tgs TagSet) {
	ckey := GrainKey(name, m, aggr)

	if _, ok := sb.Counts[ckey]; !ok {
		sb.Counts[ckey] = NewCount(m, ckey, name, tgs)
	}

	sb.Counts[ckey] = sb.Counts[ckey].Add(v)
}

func (sb StatsBucket) addToDistribution(m string, v float64, sampleID uint64, aggr, name string, tgs TagSet) {
	ckey := GrainKey(name, m, aggr)

	if _, ok := sb.Distributions[ckey]; !ok {
		sb.Distributions[ckey] = NewDistribution(m, ckey, name, tgs)
	}

	sb.Distributions[ckey].Add(v, sampleID)
}

// IsEmpty just says if this stats bucket has no information (in which case it's useless)
func (sb StatsBucket) IsEmpty() bool {
	return len(sb.Counts) == 0 && len(sb.Distributions) == 0
}

// 10 bits precision (any value will be +/- 1/1024)
const roundMask int64 = 1 << 10

// nsTimestampToFloat converts a nanosec timestamp into a float nanosecond timestamp truncated to a fixed precision
func nsTimestampToFloat(ns int64) float64 {
	var shift uint
	for ns > roundMask {
		ns = ns >> 1
		shift++
	}
	return float64(ns << shift)
}

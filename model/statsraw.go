package model

import (
	"bytes"
	"sort"

	"github.com/DataDog/datadog-trace-agent/quantile"
)

// Most "algorithm" stuff here is tested with stats_test.go as what is important
// is that the final data, the one with send after a call to Export(), is correct.

type groupedStats struct {
	tgs                  TagSet
	hitsCount            int64
	errorsCount          int64
	durationCount        int64
	durationDistribution *quantile.SliceSummary
}

func newGroupedStats(tgs TagSet) groupedStats {
	return groupedStats{
		tgs:                  tgs,
		durationDistribution: quantile.NewSliceSummary(),
	}
}

type statsKey struct {
	name string
	aggr string
}

// StatsRawBucket is used to compute span data and aggregate it
// within a time-framed bucket. This should not be used outside
// the agent, use StatsBucket for this.
type StatsRawBucket struct {
	// This should really have no public fields. At all.

	start    int64 // timestamp of start in our format
	duration int64 // duration of a bucket in nanoseconds

	// this should really remain private as it's subject to refactoring
	data map[statsKey]groupedStats

	// internal buffer for aggregate strings - not threadsafe
	keyBuf bytes.Buffer
}

// NewStatsRawBucket opens a new calculation bucket for time ts and initializes it properly
func NewStatsRawBucket(ts, d int64) StatsRawBucket {
	// The only non-initialized value is the Duration which should be set by whoever closes that bucket
	return StatsRawBucket{
		start:    ts,
		duration: d,
		data:     make(map[statsKey]groupedStats),
	}
}

// Export transforms a StatsRawBucket into a StatsBucket, typically used
// before communicating data to the API, as StatsRawBucket is the internal
// type while StatsBucket is the public, shared one.
func (sb *StatsRawBucket) Export() StatsBucket {
	ret := NewStatsBucket(sb.start, sb.duration)
	for k, v := range sb.data {
		hitsKey := GrainKey(k.name, HITS, k.aggr)
		ret.Counts[hitsKey] = Count{
			Key:     hitsKey,
			Name:    k.name,
			Measure: HITS,
			TagSet:  v.tgs,
			Value:   float64(v.hitsCount),
		}
		errorsKey := GrainKey(k.name, ERRORS, k.aggr)
		ret.Counts[errorsKey] = Count{
			Key:     errorsKey,
			Name:    k.name,
			Measure: ERRORS,
			TagSet:  v.tgs,
			Value:   float64(v.errorsCount),
		}
		durationKey := GrainKey(k.name, DURATION, k.aggr)
		ret.Counts[durationKey] = Count{
			Key:     durationKey,
			Name:    k.name,
			Measure: DURATION,
			TagSet:  v.tgs,
			Value:   float64(v.durationCount),
		}
		ret.Distributions[durationKey] = Distribution{
			Key:     durationKey,
			Name:    k.name,
			Measure: DURATION,
			TagSet:  v.tgs,
			Summary: v.durationDistribution,
		}
	}
	return ret
}

func assembleGrain(b *bytes.Buffer, env, resource, service string, m map[string]string) (string, TagSet) {
	b.Reset()

	b.WriteString("env:")
	b.WriteString(env)
	b.WriteString(",resource:")
	b.WriteString(resource)
	b.WriteString(",service:")
	b.WriteString(service)

	tagset := TagSet{{"env", env}, {"resource", resource}, {"service", service}}

	if m == nil || len(m) == 0 {
		return b.String(), tagset
	}

	keys := make([]string, len(m))
	j := 0
	for k := range m {
		keys[j] = k
		j++
	}

	sort.Strings(keys) // required else aggregations would not work

	for _, key := range keys {
		b.WriteRune(',')
		b.WriteString(key)
		b.WriteRune(':')
		b.WriteString(m[key])
		tagset = append(tagset, Tag{key, m[key]})
	}

	return b.String(), tagset
}

// HandleSpan adds the span to this bucket stats, aggregated with the finest grain matching given aggregators
func (sb *StatsRawBucket) HandleSpan(s Span, env string, aggregators []string, sublayers *[]SublayerValue) {
	if env == "" {
		panic("env should never be empty")
	}

	m := make(map[string]string)

	for _, agg := range aggregators {
		if agg != "env" && agg != "resource" && agg != "service" {
			if v, ok := s.Meta[agg]; ok {
				m[agg] = v
			}
		}
	}

	grain, tags := assembleGrain(&sb.keyBuf, env, s.Resource, s.Service, m)
	sb.add(s, grain, tags)

	// sublayers - special case
	if sublayers != nil {
		for _, sub := range *sublayers {
			sb.addSublayer(s, grain, tags, sub)
		}
	}
}

func (sb StatsRawBucket) add(s Span, aggr string, tgs TagSet) {
	var gs groupedStats
	var ok bool

	key := statsKey{name: s.Name, aggr: aggr}
	if gs, ok = sb.data[key]; !ok {
		gs = newGroupedStats(tgs)
	}

	gs.hitsCount++
	if s.Error != 0 {
		gs.errorsCount++
	}
	gs.durationCount += s.Duration

	// TODO add for s.Metrics ability to define arbitrary counts and distros, check some config?
	// alter resolution of duration distro
	trundur := nsTimestampToFloat(s.Duration)
	gs.durationDistribution.Insert(trundur, s.SpanID)

	sb.data[key] = gs
}

func (sb StatsRawBucket) addSublayer(s Span, aggr string, tgs TagSet, sub SublayerValue) {
	// This is not as efficient as a "regular" add as we don't update
	// all sublayers at once (one call for HITS, and another one for ERRORS, DURATION...)
	// when logically, if we have a sublayer for HITS, we also have one for DURATION,
	// they should indeed come together. Still room for improvement here.

	var gs groupedStats
	var ok bool

	subAggr := aggr + "," + sub.Tag.Name + ":" + sub.Tag.Value
	subTgs := make(TagSet, len(tgs)+1)
	copy(subTgs, tgs)
	subTgs[len(tgs)] = sub.Tag

	key := statsKey{name: s.Name, aggr: subAggr}
	if gs, ok = sb.data[key]; !ok {
		gs = newGroupedStats(subTgs)
	}

	switch sub.Metric {
	case HITS:
		gs.hitsCount += int64(sub.Value)
	case ERRORS:
		gs.errorsCount += int64(sub.Value)
	case DURATION:
		gs.durationCount += int64(sub.Value)
	}

	sb.data[key] = gs
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

package main

import (
	"math/rand"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

var testBucketInterval = time.Duration(2 * time.Second).Nanoseconds()

func NewTestConcentrator() *Concentrator {
	return NewConcentrator([]string{}, time.Second.Nanoseconds())
}

// getTsInBucket gives a timestamp in ns which is `offset` buckets late
func getTsInBucket(alignedNow int64, bsize int64, offset int64) int64 {
	return alignedNow - offset*bsize + rand.Int63n(bsize)
}

// testSpan avoids typo and inconsistency in test spans (typical pitfall: duration, start time,
// and end time are aligned, and end time is the one that needs to be aligned
func testSpan(c *Concentrator, spanID uint64, duration, offset int64, service, resource string, err int32) *model.Span {
	now := model.Now()
	alignedNow := now - now%c.bsize

	return &model.Span{
		SpanID:   spanID,
		Duration: duration,
		Start:    getTsInBucket(alignedNow, c.bsize, offset) - duration,
		Service:  service,
		Name:     "query",
		Resource: resource,
		Error:    err,
	}
}

func TestConcentratorStatsCounts(t *testing.T) {
	assert := assert.New(t)
	c := NewConcentrator([]string{}, testBucketInterval)

	now := model.Now()
	alignedNow := now - now%c.bsize

	trace := model.Trace{
		// first bucket
		testSpan(c, 1, 24, 3, "A1", "resource1", 0),
		testSpan(c, 2, 12, 3, "A1", "resource1", 2),
		testSpan(c, 3, 40, 3, "A2", "resource2", 2),
		testSpan(c, 4, 300000000000, 3, "A2", "resource2", 2), // 5 minutes trace
		testSpan(c, 5, 30, 3, "A2", "resourcefoo", 0),
		// second bucket
		testSpan(c, 6, 24, 2, "A1", "resource2", 0),
		testSpan(c, 7, 12, 2, "A1", "resource1", 2),
		testSpan(c, 8, 40, 2, "A2", "resource1", 2),
		testSpan(c, 9, 30, 2, "A2", "resource2", 2),
		testSpan(c, 10, 3600000000000, 2, "A2", "resourcefoo", 0), // 1 hour trace
		// third bucket - but should not be flushed because it's the second to last
		testSpan(c, 6, 24, 1, "A1", "resource2", 0),
	}
	trace.ComputeTopLevel()
	wt := model.NewWeightedTrace(trace, trace.GetRoot())

	testTrace := processedTrace{
		Env:           "none",
		Trace:         trace,
		WeightedTrace: wt,
	}

	c.Add(testTrace)
	stats := c.Flush()

	if !assert.Equal(2, len(stats), "We should get exactly 2 StatsBucket") {
		t.FailNow()
	}

	// nothing guarantees the order of the buckets, they're from a map
	var receivedBuckets []model.StatsBucket
	if stats[0].Start < stats[1].Start {
		receivedBuckets = []model.StatsBucket{stats[0], stats[1]}
	} else {
		receivedBuckets = []model.StatsBucket{stats[1], stats[0]}
	}

	// inspect our 2 stats buckets
	assert.Equal(alignedNow-3*testBucketInterval, receivedBuckets[0].Start)
	assert.Equal(alignedNow-2*testBucketInterval, receivedBuckets[1].Start)

	var receivedCounts map[string]model.Count

	// Start with the first/older bucket
	receivedCounts = receivedBuckets[0].Counts
	expectedCountValByKey := map[string]int64{
		"query|duration|env:none,resource:resource1,service:A1":   36,
		"query|duration|env:none,resource:resource2,service:A2":   300000000040,
		"query|duration|env:none,resource:resourcefoo,service:A2": 30,
		"query|errors|env:none,resource:resource1,service:A1":     1,
		"query|errors|env:none,resource:resource2,service:A2":     2,
		"query|errors|env:none,resource:resourcefoo,service:A2":   0,
		"query|hits|env:none,resource:resource1,service:A1":       2,
		"query|hits|env:none,resource:resource2,service:A2":       2,
		"query|hits|env:none,resource:resourcefoo,service:A2":     1,
	}

	// FIXME[leo]: assert distributions!
	// verify we got all counts
	assert.Equal(len(expectedCountValByKey), len(receivedCounts), "GOT %v", receivedCounts)
	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedCounts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, int64(count.Value), "Wrong value for count %s", key)
	}

	// same for second bucket
	receivedCounts = receivedBuckets[1].Counts
	expectedCountValByKey = map[string]int64{
		"query|duration|env:none,resource:resource1,service:A1":   12,
		"query|duration|env:none,resource:resource2,service:A1":   24,
		"query|duration|env:none,resource:resource1,service:A2":   40,
		"query|duration|env:none,resource:resource2,service:A2":   30,
		"query|duration|env:none,resource:resourcefoo,service:A2": 3600000000000,
		"query|errors|env:none,resource:resource1,service:A1":     1,
		"query|errors|env:none,resource:resource2,service:A1":     0,
		"query|errors|env:none,resource:resource1,service:A2":     1,
		"query|errors|env:none,resource:resource2,service:A2":     1,
		"query|errors|env:none,resource:resourcefoo,service:A2":   0,
		"query|hits|env:none,resource:resource1,service:A1":       1,
		"query|hits|env:none,resource:resource2,service:A1":       1,
		"query|hits|env:none,resource:resource1,service:A2":       1,
		"query|hits|env:none,resource:resource2,service:A2":       1,
		"query|hits|env:none,resource:resourcefoo,service:A2":     1,
	}

	// verify we got all counts
	assert.Equal(len(expectedCountValByKey), len(receivedCounts), "GOT %v", receivedCounts)
	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedCounts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, int64(count.Value), "Wrong value for count %s", key)
	}
}

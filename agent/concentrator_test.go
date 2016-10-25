package main

import (
	"math/rand"
	"testing"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func NewTestConcentrator() *Concentrator {
	conf := config.NewDefaultAgentConfig()
	conf.BucketInterval = time.Duration(1) * time.Second

	in := make(chan model.Trace)

	return NewConcentrator(in, conf)
}

// getTsInBucket gives a timestamp in ns which is `offset` buckets late
func getTsInBucket(alignedNow int64, bucketInterval time.Duration, offset int64) int64 {
	return alignedNow - offset*bucketInterval.Nanoseconds() + rand.Int63n(bucketInterval.Nanoseconds())
}

// testSpan avoids typo and inconsistency in test spans (typical pitfall: duration, start time,
// and end time are aligned, and end time is the one that needs to be aligned
func testSpan(c *Concentrator, spanID uint64, duration, offset int64, service, resource string, err int32) model.Span {
	bucketInterval := c.conf.BucketInterval
	now := model.Now()
	alignedNow := now - now%bucketInterval.Nanoseconds()

	return model.Span{
		SpanID:   spanID,
		Duration: duration,
		Start:    getTsInBucket(alignedNow, bucketInterval, offset) - duration,
		Service:  service,
		Name:     "query",
		Resource: resource,
		Error:    err,
	}
}

func TestConcentratorStatsCounts(t *testing.T) {
	assert := assert.New(t)

	c := NewTestConcentrator()
	defer close(c.in)

	// accept all the spans by hacking the cutoff
	c.conf.OldestSpanCutoff = time.Minute.Nanoseconds()

	bucketInterval := c.conf.BucketInterval
	now := model.Now()
	alignedNow := now - now%bucketInterval.Nanoseconds()

	testSpans := model.Trace{
		// first bucket
		testSpan(c, 1, 24, 2, "A1", "resource1", 0),
		testSpan(c, 2, 12, 2, "A1", "resource1", 2),
		testSpan(c, 3, 40, 2, "A2", "resource2", 2),
		testSpan(c, 4, 300000000000, 2, "A2", "resource2", 2), // 5 minutes trace
		testSpan(c, 5, 30, 2, "A2", "resourcefoo", 0),
		testSpan(c, 6, 24, 1, "A1", "resource2", 0),
		testSpan(c, 7, 12, 1, "A1", "resource1", 2),
		testSpan(c, 8, 40, 1, "A2", "resource1", 2),
		testSpan(c, 9, 30, 1, "A2", "resource2", 2),
		testSpan(c, 10, 3600000000000, 1, "A2", "resourcefoo", 0), // 1 hour trace
	}

	go c.Run()

	// insert the spans
	c.in <- testSpans

	// Restore the correct cutoff after being sure we processed all the spans
	time.Sleep(10 * time.Millisecond)
	c.conf.OldestSpanCutoff = time.Second.Nanoseconds()

	// Triggers the flush
	c.in <- model.NewTraceFlushMarker()

	// Get the stats from the flush
	stats := <-c.out

	c.in <- model.Trace{testSpan(c, 100, 1, 0, "A1", "resource1", 0)}
	c.in <- model.Trace{testSpan(c, 101, 1, 1, "A1", "resource1", 0)}
	c.in <- model.Trace{testSpan(c, 102, 1, 2, "A1", "resource1", 0)}

	if !assert.Equal(len(stats), 2, "We should get exactly 2 StatsBucket") {
		t.FailNow()
	}

	receivedBuckets := []model.StatsBucket{stats[0], stats[1]}

	// inspect our 2 stats buckets
	assert.Equal(alignedNow-2*bucketInterval.Nanoseconds(), receivedBuckets[0].Start)
	assert.Equal(alignedNow-bucketInterval.Nanoseconds(), receivedBuckets[1].Start)

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

// TODO[leo] test extra aggregators here?

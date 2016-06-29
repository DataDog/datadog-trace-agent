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
		model.Span{SpanID: 1, Duration: 24, Start: getTsInBucket(alignedNow, bucketInterval, 2), Service: "A1", Name: "query", Resource: "resource1", Weight: 1},
		model.Span{SpanID: 2, Duration: 12, Start: getTsInBucket(alignedNow, bucketInterval, 2), Service: "A1", Name: "query", Resource: "resource1", Error: 2, Weight: 4},
		model.Span{SpanID: 3, Duration: 40, Start: getTsInBucket(alignedNow, bucketInterval, 2), Service: "A2", Name: "query", Resource: "resource2", Error: 2, Weight: 1},
		model.Span{SpanID: 4, Duration: 30, Start: getTsInBucket(alignedNow, bucketInterval, 2), Service: "A2", Name: "query", Resource: "resource2", Error: 2},
		model.Span{SpanID: 5, Duration: 30, Start: getTsInBucket(alignedNow, bucketInterval, 2), Service: "A2", Name: "query", Resource: "resourcefoo"},
		// second bucket
		model.Span{SpanID: 6, Duration: 24, Start: getTsInBucket(alignedNow, bucketInterval, 1), Service: "A1", Name: "query", Resource: "resource2", Weight: 2.5},
		model.Span{SpanID: 7, Duration: 12, Start: getTsInBucket(alignedNow, bucketInterval, 1), Service: "A1", Name: "query", Resource: "resource1", Error: 2},
		model.Span{SpanID: 8, Duration: 40, Start: getTsInBucket(alignedNow, bucketInterval, 1), Service: "A2", Name: "query", Resource: "resource1", Error: 2},
		model.Span{SpanID: 9, Duration: 30, Start: getTsInBucket(alignedNow, bucketInterval, 1), Service: "A2", Name: "query", Resource: "resource2", Error: 2, Weight: 1},
		model.Span{SpanID: 10, Duration: 20, Start: getTsInBucket(alignedNow, bucketInterval, 1), Service: "A2", Name: "query", Resource: "resourcefoo"},
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

	c.in <- model.Trace{model.Span{SpanID: 100, Duration: 1, Start: getTsInBucket(alignedNow, bucketInterval, 0), Service: "A1", Name: "query", Resource: "resource1"}}
	c.in <- model.Trace{model.Span{SpanID: 101, Duration: 1, Start: getTsInBucket(alignedNow, bucketInterval, 1), Service: "A1", Name: "query", Resource: "resource1"}}
	c.in <- model.Trace{model.Span{SpanID: 102, Duration: 1, Start: getTsInBucket(alignedNow, bucketInterval, 2), Service: "A1", Name: "query", Resource: "resource1"}}

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
	expectedCountValByKey := map[string]float64{
		"duration|name:query,resource:resource1,service:A1":   48+24,
		"duration|name:query,resource:resource2,service:A2":   70,
		"duration|name:query,resource:resourcefoo,service:A2": 30,
		"errors|name:query,resource:resource1,service:A1":     4,
		"errors|name:query,resource:resource2,service:A2":     2,
		"errors|name:query,resource:resourcefoo,service:A2":   0,
		"hits|name:query,resource:resource1,service:A1":       5,
		"hits|name:query,resource:resource2,service:A2":       2,
		"hits|name:query,resource:resourcefoo,service:A2":     1,
	}

	// FIXME[leo]: assert distributions!
	// verify we got all counts
	assert.Equal(len(expectedCountValByKey), len(receivedCounts), "GOT %v", receivedCounts)
	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedCounts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, count.Value, "Wrong value for count %s", key)
	}

	// same for second bucket
	receivedCounts = receivedBuckets[1].Counts
	expectedCountValByKey = map[string]float64{
		"duration|name:query,resource:resource1,service:A1":   12,
		"duration|name:query,resource:resource2,service:A1":   60,
		"duration|name:query,resource:resource1,service:A2":   40,
		"duration|name:query,resource:resource2,service:A2":   30,
		"duration|name:query,resource:resourcefoo,service:A2": 20,
		"errors|name:query,resource:resource1,service:A1":     1,
		"errors|name:query,resource:resource2,service:A1":     0,
		"errors|name:query,resource:resource1,service:A2":     1,
		"errors|name:query,resource:resource2,service:A2":     1,
		"errors|name:query,resource:resourcefoo,service:A2":   0,
		"hits|name:query,resource:resource1,service:A1":       1,
		"hits|name:query,resource:resource2,service:A1":       2.5,
		"hits|name:query,resource:resource1,service:A2":       1,
		"hits|name:query,resource:resource2,service:A2":       1,
		"hits|name:query,resource:resourcefoo,service:A2":     1,
	}

	// verify we got all counts
	assert.Equal(len(expectedCountValByKey), len(receivedCounts), "GOT %v", receivedCounts)
	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedCounts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, count.Value, "Wrong value for count %s", key)
	}
}

// TODO[leo] test extra aggregators here?

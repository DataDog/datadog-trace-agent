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

	in := make(chan model.Span)

	return NewConcentrator(in, conf)
}

func TestConcentratorExitsGracefully(t *testing.T) {
	c := NewTestConcentrator()
	c.Start()

	// Try to stop it in a given time, by closing the exit channel
	timer := time.NewTimer(100 * time.Millisecond).C
	receivedExit := make(chan struct{}, 1)
	go func() {
		close(c.exit)
		c.wg.Wait()
		close(receivedExit)
	}()
	for {
		select {
		case <-receivedExit:
			return
		case <-timer:
			t.Fatal("Concentrator did not exit in time")
		}
	}
}

// getTsInBucket gives a timestamp in ns which is `offset` buckets late
func getTsInBucket(alignedNow int64, bucketInterval time.Duration, offset int64) int64 {
	return alignedNow - offset*bucketInterval.Nanoseconds() + rand.Int63n(bucketInterval.Nanoseconds())
}

func TestConcentratorStatsCounts(t *testing.T) {
	assert := assert.New(t)

	c := NewTestConcentrator()

	// accept all the spans by hacking the cutoff
	c.conf.OldestSpanCutoff = time.Minute.Nanoseconds()

	bucketInterval := c.conf.BucketInterval
	now := model.Now()
	alignedNow := now - now%bucketInterval.Nanoseconds()

	testSpans := []model.Span{
		// first bucket
		model.Span{SpanID: 1, Duration: 24, Start: getTsInBucket(alignedNow, bucketInterval, 2), Layer: "service1", Resource: "resource1"},
		model.Span{SpanID: 2, Duration: 12, Start: getTsInBucket(alignedNow, bucketInterval, 2), Layer: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 3, Duration: 40, Start: getTsInBucket(alignedNow, bucketInterval, 2), Layer: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 4, Duration: 30, Start: getTsInBucket(alignedNow, bucketInterval, 2), Layer: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 5, Duration: 30, Start: getTsInBucket(alignedNow, bucketInterval, 2), Layer: "service2", Resource: "resourcefoo"},
		// second bucket
		model.Span{SpanID: 6, Duration: 24, Start: getTsInBucket(alignedNow, bucketInterval, 1), Layer: "service1", Resource: "resource2"},
		model.Span{SpanID: 7, Duration: 12, Start: getTsInBucket(alignedNow, bucketInterval, 1), Layer: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 8, Duration: 40, Start: getTsInBucket(alignedNow, bucketInterval, 1), Layer: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 9, Duration: 30, Start: getTsInBucket(alignedNow, bucketInterval, 1), Layer: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 10, Duration: 20, Start: getTsInBucket(alignedNow, bucketInterval, 1), Layer: "service2", Resource: "resourcefoo"},
	}

	c.Start()

	// insert the spans
	for _, s := range testSpans {
		c.in <- s
	}

	// Restore the correct cutoff after being sure we processed all the spans
	time.Sleep(10 * time.Millisecond)
	c.conf.OldestSpanCutoff = time.Second.Nanoseconds()

	// Triggers the flush
	c.in <- model.NewFlushMarker()

	// Send several spans which shouldn't be considered by this flush
	go func() {
		c.in <- model.Span{SpanID: 100, Duration: 1, Start: getTsInBucket(alignedNow, bucketInterval, 0), Layer: "service1", Resource: "resource1"}
		c.in <- model.Span{SpanID: 101, Duration: 1, Start: getTsInBucket(alignedNow, bucketInterval, 1), Layer: "service1", Resource: "resource1"}
		c.in <- model.Span{SpanID: 102, Duration: 1, Start: getTsInBucket(alignedNow, bucketInterval, 2), Layer: "service1", Resource: "resource1"}
	}()

	// Get the stats from the flush
	stats := <-c.out

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
	t.Log(receivedCounts)
	expectedCountValByKey := map[string]int64{
		"hits|app:service1,resource:resource1":       2,
		"hits|app:service1,resource:resource2":       2,
		"hits|app:service2,resource:resourcefoo":     1,
		"errors|app:service1,resource:resource1":     1,
		"errors|app:service1,resource:resource2":     2,
		"errors|app:service2,resource:resourcefoo":   0,
		"duration|app:service1,resource:resource1":   36,
		"duration|app:service1,resource:resource2":   70,
		"duration|app:service2,resource:resourcefoo": 30,
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
	t.Log(receivedCounts)
	expectedCountValByKey = map[string]int64{
		"hits|app:service1,resource:resource1":       2,
		"hits|app:service1,resource:resource2":       2,
		"hits|app:service2,resource:resourcefoo":     1,
		"errors|app:service1,resource:resource1":     2,
		"errors|app:service1,resource:resource2":     1,
		"errors|app:service2,resource:resourcefoo":   0,
		"duration|app:service1,resource:resource1":   52,
		"duration|app:service1,resource:resource2":   54,
		"duration|app:service2,resource:resourcefoo": 20,
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

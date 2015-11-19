package main

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func NewTestConcentrator() *Concentrator {
	exit := make(chan struct{})
	var exitGroup sync.WaitGroup

	conf := config.NewDefaultAgentConfig()
	conf.BucketInterval = time.Duration(1) * time.Second

	in := make(chan model.Span)

	return NewConcentrator(
		in,
		conf,
		exit,
		&exitGroup,
	)
}

func TestConcentratorExitsGracefully(t *testing.T) {
	// Start a concentrator
	c := NewTestConcentrator()
	c.Start()

	// And now try to stop it in a given time, by closing the exit channel
	timer := time.NewTimer(100 * time.Millisecond).C
	receivedExit := make(chan struct{}, 1)
	go func() {
		close(c.exit)
		c.exitGroup.Wait()
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

// getTsInBucket(now(), 1s, 3) get you a nanosecond timestamp, 3 buckets later from now (buckets aligned on 1s)
func getTsInBucket(ref int64, bucketInterval time.Duration, offset int64) int64 {
	// align it on bucket
	ref = ref - ref%bucketInterval.Nanoseconds()

	return ref + offset*bucketInterval.Nanoseconds() + rand.Int63n(bucketInterval.Nanoseconds())
}

func TestConcentratorStatsCounts(t *testing.T) {
	assert := assert.New(t)

	c := NewTestConcentrator()
	// we want this faster
	c.conf.OldestSpanCutoff = time.Second.Nanoseconds()

	now := model.Now()

	bucketInterval := c.conf.BucketInterval

	testSpans := []model.Span{
		// first bucket
		model.Span{SpanID: 1, Duration: 24, Start: getTsInBucket(now, bucketInterval, 0), Service: "service1", Resource: "resource1"},
		model.Span{SpanID: 2, Duration: 12, Start: getTsInBucket(now, bucketInterval, 0), Service: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 3, Duration: 40, Start: getTsInBucket(now, bucketInterval, 0), Service: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 4, Duration: 30, Start: getTsInBucket(now, bucketInterval, 0), Service: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 5, Duration: 30, Start: getTsInBucket(now, bucketInterval, 0), Service: "service2", Resource: "resourcefoo"},
		// second bucket
		model.Span{SpanID: 6, Duration: 24, Start: getTsInBucket(now, bucketInterval, 1), Service: "service1", Resource: "resource2"},
		model.Span{SpanID: 7, Duration: 12, Start: getTsInBucket(now, bucketInterval, 1), Service: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 8, Duration: 40, Start: getTsInBucket(now, bucketInterval, 1), Service: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 9, Duration: 30, Start: getTsInBucket(now, bucketInterval, 1), Service: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 10, Duration: 30, Start: getTsInBucket(now, bucketInterval, 1), Service: "service2", Resource: "resourcefoo"},
	}

	c.Start()
	// we should expect 2 buckets!
	receivedBuckets := make([]model.StatsBucket, 0, 2)

	// we have to wait at least for the 2 buckets to be "flushable", ie. now - c.conf.OldestSpanCutoff is older than their ts
	maxWaitFlushTimer := time.NewTimer(time.Duration(c.conf.OldestSpanCutoff)*time.Nanosecond + 2*bucketInterval).C
	waitingForBucket := make(chan struct{})
	go func() {
		for {
			select {
			case <-maxWaitFlushTimer:
				close(waitingForBucket)
				break
			case payload := <-c.out:
				receivedBuckets = append(receivedBuckets, payload.Stats)
			}
		}
	}()

	// insert the spans!
	for _, span := range testSpans {
		c.in <- span
	}

	<-waitingForBucket
	// FIXME[leo]: assert something in the sampler?
	if !assert.Equal(2, len(receivedBuckets)) {
		// Don't bother continuing
		t.FailNow()
	}
	// inspect our 2 stats buckets
	assert.Equal(now-now%bucketInterval.Nanoseconds(), receivedBuckets[0].Start)
	assert.Equal(now-now%bucketInterval.Nanoseconds()+bucketInterval.Nanoseconds(), receivedBuckets[1].Start)

	expectedCountValByKey := map[string]int64{
		"hits|resource:resource1,service:service1":       2,
		"hits|resource:resource2,service:service1":       2,
		"hits|resource:resourcefoo,service:service2":     1,
		"errors|resource:resource1,service:service1":     1,
		"errors|resource:resource2,service:service1":     2,
		"errors|resource:resourcefoo,service:service2":   0,
		"duration|resource:resource1,service:service1":   36,
		"duration|resource:resource2,service:service1":   70,
		"duration|resource:resourcefoo,service:service2": 30,
	}

	// FIXME[leo]: assert distributions!
	// verify we got all counts
	assert.Equal(len(expectedCountValByKey), len(receivedBuckets[0].Counts), "GOT %v", receivedBuckets[0].Counts)
	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedBuckets[0].Counts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, count.Value, "Wrong value for count %s", key)
	}

	// same for second bucket
	expectedCountValByKey = map[string]int64{
		"hits|resource:resource1,service:service1":       2,
		"hits|resource:resource2,service:service1":       2,
		"hits|resource:resourcefoo,service:service2":     1,
		"errors|resource:resource1,service:service1":     2,
		"errors|resource:resource2,service:service1":     1,
		"errors|resource:resourcefoo,service:service2":   0,
		"duration|resource:resource1,service:service1":   52,
		"duration|resource:resource2,service:service1":   54,
		"duration|resource:resourcefoo,service:service2": 30,
	}

	// verify we got all counts
	assert.Equal(len(expectedCountValByKey), len(receivedBuckets[1].Counts), "GOT %v", receivedBuckets[1].Counts)
	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedBuckets[1].Counts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, count.Value, "Wrong value for count %s", key)
	}
}

// TODO[leo] test extra aggregators here?

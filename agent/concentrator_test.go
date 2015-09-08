package main

import (
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func NewTestConcentrator(inSpans chan model.Span) (*Concentrator, chan model.Span, chan model.StatsBucket) {
	exit := make(chan struct{})
	var exitGroup sync.WaitGroup

	return NewConcentrator(
		time.Second,
		inSpans,
		exit,
		&exitGroup,
	)
}

func WaitForSpanOnChan(c *Concentrator, channel chan model.Span, timeout time.Duration) (model.Span, error) {
	timeBomb := time.NewTimer(timeout).C
	for {
		select {
		case <-timeBomb:
			return model.Span{}, errors.New("Did not receive span in time")
		case processed := <-c.outSpans:
			return processed, nil
		}
	}
}

func TestConcentratorExitsGracefully(t *testing.T) {
	// Start a concentrator
	inSpans := make(chan model.Span)
	c, _, _ := NewTestConcentrator(inSpans)
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

func TestConcentratorRejectsLateSpan(t *testing.T) {
	assert := assert.New(t)

	inSpans := make(chan model.Span)
	c, outSpans, _ := NewTestConcentrator(inSpans)

	c.Start()

	s := model.Span{}
	inSpans <- s

	// span with s.Start = 0 should be filtered out by concentrator
	_, err := WaitForSpanOnChan(c, outSpans, 100*time.Millisecond)
	assert.NotNil(err)
}

func TestSpanPassesThroughConcentrator(t *testing.T) {
	assert := assert.New(t)

	inSpans := make(chan model.Span)
	c, outSpans, _ := NewTestConcentrator(inSpans)

	c.Start()

	s := model.Span{SpanID: 42, Start: model.Now()}
	inSpans <- s

	// span with s.Start = 0 should be filtered out by concentrator
	receivedSpan, err := WaitForSpanOnChan(c, outSpans, 100*time.Millisecond)
	assert.Nil(err)
	assert.Equal(s.SpanID, receivedSpan.SpanID)
}

// getTsInBucket(now(), 1s, 3) get you a nanosecond timestamp, 3 buckets later from now (buckets aligned on 1s)
func getTsInBucket(ref int64, bucketSize time.Duration, offset int64) int64 {
	// align it on bucket
	ref = ref - ref%bucketSize.Nanoseconds()

	return ref + offset*bucketSize.Nanoseconds() + rand.Int63n(bucketSize.Nanoseconds())
}

func TestConcentratorStatsCounts(t *testing.T) {
	assert := assert.New(t)

	inSpans := make(chan model.Span)
	c, outSpans, outStats := NewTestConcentrator(inSpans)
	// we want this faster
	c.oldestSpanCutoff = time.Second.Nanoseconds()

	now := model.Now()

	testSpans := []model.Span{
		// first bucket
		model.Span{SpanID: 1, Duration: 24, Start: getTsInBucket(now, c.bucketSize, 0), Service: "service1", Resource: "resource1"},
		model.Span{SpanID: 2, Duration: 12, Start: getTsInBucket(now, c.bucketSize, 0), Service: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 3, Duration: 40, Start: getTsInBucket(now, c.bucketSize, 0), Service: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 4, Duration: 30, Start: getTsInBucket(now, c.bucketSize, 0), Service: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 5, Duration: 30, Start: getTsInBucket(now, c.bucketSize, 0), Service: "service2", Resource: "resourcefoo"},
		// second bucket
		model.Span{SpanID: 6, Duration: 24, Start: getTsInBucket(now, c.bucketSize, 1), Service: "service1", Resource: "resource2"},
		model.Span{SpanID: 7, Duration: 12, Start: getTsInBucket(now, c.bucketSize, 1), Service: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 8, Duration: 40, Start: getTsInBucket(now, c.bucketSize, 1), Service: "service1", Resource: "resource1", Error: 2},
		model.Span{SpanID: 9, Duration: 30, Start: getTsInBucket(now, c.bucketSize, 1), Service: "service1", Resource: "resource2", Error: 2},
		model.Span{SpanID: 10, Duration: 30, Start: getTsInBucket(now, c.bucketSize, 1), Service: "service2", Resource: "resourcefoo"},
	}

	c.Start()
	receivedStats := make([]model.StatsBucket, 0, 2)
	receivedSpans := make([]model.Span, 0, len(testSpans))

	// we have to wait at least for the 2 buckets to be "flushable", ie. now - c.oldestSpanCutoff is older than their ts
	maxWaitFlushTimer := time.NewTimer(time.Duration(c.oldestSpanCutoff)*time.Nanosecond + 2*c.bucketSize).C
	waitForStats := make(chan bool)
	go func() {
		for {
			select {
			case <-maxWaitFlushTimer:
				waitForStats <- true
				break
			case stats := <-outStats:
				receivedStats = append(receivedStats, stats)
			case span := <-outSpans:
				receivedSpans = append(receivedSpans, span)
			}
		}
	}()

	// insert the spans!
	for _, span := range testSpans {
		inSpans <- span
	}

	<-waitForStats
	assert.Equal(testSpans, receivedSpans)
	if !assert.Equal(2, len(receivedStats)) {
		// Don't bother continuing
		t.FailNow()
	}
	// inspect our 2 stats buckets
	assert.Equal(now-now%c.bucketSize.Nanoseconds(), receivedStats[0].Start)
	assert.Equal(now-now%c.bucketSize.Nanoseconds()+c.bucketSize.Nanoseconds(), receivedStats[1].Start)

	expectedCountValByKey := map[string]int64{
		"hits|service:service1":                          4,
		"hits|service:service2":                          1,
		"errors|service:service1":                        3,
		"errors|service:service2":                        0,
		"duration|service:service1":                      106,
		"duration|service:service2":                      30,
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

	// verify we got all counts
	assert.Equal(len(expectedCountValByKey), len(receivedStats[0].Counts), "GOT %v", receivedStats[0].Counts)
	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedStats[0].Counts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, count.Value, "Wrong value for count %s", key)
	}

	// same for second bucket
	expectedCountValByKey = map[string]int64{
		"hits|service:service1":                          4,
		"hits|service:service2":                          1,
		"errors|service:service1":                        3,
		"errors|service:service2":                        0,
		"duration|service:service1":                      106,
		"duration|service:service2":                      30,
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
	assert.Equal(len(expectedCountValByKey), len(receivedStats[1].Counts), "GOT %v", receivedStats[1].Counts)
	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedStats[1].Counts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, count.Value, "Wrong value for count %s", key)
	}
}

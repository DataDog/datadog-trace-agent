package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

var testBucketInterval = time.Duration(2 * time.Second).Nanoseconds()

func NewTestConcentrator() *Concentrator {
	statsChan := make(chan []model.StatsBucket)
	return NewConcentrator([]string{}, time.Second.Nanoseconds(), statsChan)
}

// getTsInBucket gives a timestamp in ns which is `offset` buckets late
func getTsInBucket(alignedNow int64, bsize int64, offset int64) int64 {
	return alignedNow - offset*bsize + rand.Int63n(bsize)
}

// testSpan avoids typo and inconsistency in test spans (typical pitfall: duration, start time,
// and end time are aligned, and end time is the one that needs to be aligned
func testSpan(spanID uint64, parentID uint64, duration, offset int64, service, resource string, err int32) *model.Span {
	now := model.Now()
	alignedNow := now - now%testBucketInterval

	return &model.Span{
		SpanID:   spanID,
		ParentID: parentID,
		Duration: duration,
		Start:    getTsInBucket(alignedNow, testBucketInterval, offset) - duration,
		Service:  service,
		Name:     "query",
		Resource: resource,
		Error:    err,
		Type:     "db",
	}
}

// TestConcentratorOldestTs tests that the Agent doesn't report time buckets from a
// time before its start
func TestConcentratorOldestTs(t *testing.T) {
	assert := assert.New(t)
	statsChan := make(chan []model.StatsBucket)

	now := model.Now()

	// Build that simply have spans spread over time windows.
	trace := model.Trace{
		testSpan(1, 0, 50, 5, "A1", "resource1", 0),
		testSpan(1, 0, 40, 4, "A1", "resource1", 0),
		testSpan(1, 0, 30, 3, "A1", "resource1", 0),
		testSpan(1, 0, 20, 2, "A1", "resource1", 0),
		testSpan(1, 0, 10, 1, "A1", "resource1", 0),
		testSpan(1, 0, 1, 0, "A1", "resource1", 0),
	}

	trace.ComputeTopLevel()
	wt := model.NewWeightedTrace(trace, trace.GetRoot())

	testTrace := model.ProcessedTrace{
		Env:           "none",
		Trace:         trace,
		WeightedTrace: wt,
	}

	t.Run("cold", func(t *testing.T) {
		// Running cold, all spans in the past should end up in the current time bucket.
		flushTime := now
		c := NewConcentrator([]string{}, testBucketInterval, statsChan)
		c.Add(testTrace)

		for i := 0; i < c.bufferLen; i++ {
			stats := c.flushNow(flushTime)
			if !assert.Equal(0, len(stats), "We should get exactly 0 StatsBucket") {
				t.FailNow()
			}
			flushTime += testBucketInterval
		}

		stats := c.flushNow(flushTime)

		if !assert.Equal(1, len(stats), "We should get exactly 1 StatsBucket") {
			t.FailNow()
		}

		// First oldest bucket aggregates old past time buckets, it should have it all.
		for key, count := range stats[0].Counts {
			if key == "query|duration|env:none,resource:resource1,service:A1" {
				assert.Equal(151, int(count.Value), "Wrong value for duration")
			}
			if key == "query|hits|env:none,resource:resource1,service:A1" {
				assert.Equal(6, int(count.Value), "Wrong value for hits")
			}
		}
	})

	t.Run("hot", func(t *testing.T) {
		flushTime := now
		c := NewConcentrator([]string{}, testBucketInterval, statsChan)
		c.oldestTs = alignTs(now, c.bsize) - int64(c.bufferLen-1)*c.bsize
		c.Add(testTrace)

		for i := 0; i < c.bufferLen-1; i++ {
			stats := c.flushNow(flushTime)
			if !assert.Equal(0, len(stats), "We should get exactly 0 StatsBucket") {
				t.FailNow()
			}
			flushTime += testBucketInterval
		}

		stats := c.flushNow(flushTime)
		if !assert.Equal(1, len(stats), "We should get exactly 1 StatsBucket") {
			t.FailNow()
		}
		flushTime += testBucketInterval

		// First oldest bucket aggregates, it should have it all except the last span.
		for key, count := range stats[0].Counts {
			if key == "query|duration|env:none,resource:resource1,service:A1" {
				assert.Equal(150, int(count.Value), "Wrong value for duration")
			}
			if key == "query|hits|env:none,resource:resource1,service:A1" {
				assert.Equal(5, int(count.Value), "Wrong value for hits")
			}
		}

		stats = c.flushNow(flushTime)
		if !assert.Equal(1, len(stats), "We should get exactly 1 StatsBucket") {
			t.FailNow()
		}

		// Stats of the last span.
		for key, count := range stats[0].Counts {
			if key == "query|duration|env:none,resource:resource1,service:A1" {
				assert.Equal(1, int(count.Value), "Wrong value for duration")
			}
			if key == "query|hits|env:none,resource:resource1,service:A1" {
				assert.Equal(1, int(count.Value), "Wrong value for hits")
			}
		}
	})
}

//TestConcentratorStatsTotals tests that the total stats are correct, independently of the
// time bucket they end up.
func TestConcentratorStatsTotals(t *testing.T) {
	assert := assert.New(t)
	statsChan := make(chan []model.StatsBucket)
	c := NewConcentrator([]string{}, testBucketInterval, statsChan)

	now := model.Now()
	alignedNow := alignTs(now, c.bsize)

	// update oldestTs as it running for quite some time, to avoid the fact that at startup
	// it only allows recent stats.
	c.oldestTs = alignedNow - int64(c.bufferLen)*c.bsize

	// Build that simply have spans spread over time windows.
	trace := model.Trace{
		testSpan(1, 0, 50, 5, "A1", "resource1", 0),
		testSpan(1, 0, 40, 4, "A1", "resource1", 0),
		testSpan(1, 0, 30, 3, "A1", "resource1", 0),
		testSpan(1, 0, 20, 2, "A1", "resource1", 0),
		testSpan(1, 0, 10, 1, "A1", "resource1", 0),
		testSpan(1, 0, 1, 0, "A1", "resource1", 0),
	}

	trace.ComputeTopLevel()
	wt := model.NewWeightedTrace(trace, trace.GetRoot())

	testTrace := model.ProcessedTrace{
		Env:           "none",
		Trace:         trace,
		WeightedTrace: wt,
	}
	c.Add(testTrace)

	var hits float64
	var duration float64

	flushTime := now
	for i := 0; i <= c.bufferLen; i++ {
		stats := c.flushNow(flushTime)

		if len(stats) == 0 {
			continue
		}

		for key, count := range stats[0].Counts {
			if key == "query|duration|env:none,resource:resource1,service:A1" {
				duration += count.Value
			}
			if key == "query|hits|env:none,resource:resource1,service:A1" {
				hits += count.Value
			}
		}
		flushTime += c.bsize
	}

	assert.Equal(hits, float64(len(trace)), "Wrong value for total hits %d", hits)
	assert.Equal(duration, float64(50+40+30+20+10+1), "Wrong value for total duration %d", duration)
}

// TestConcentratorStatsCounts tests exhaustively each stats bucket, over multiple time buckets.
func TestConcentratorStatsCounts(t *testing.T) {
	assert := assert.New(t)
	statsChan := make(chan []model.StatsBucket)
	c := NewConcentrator([]string{}, testBucketInterval, statsChan)

	now := model.Now()
	alignedNow := alignTs(now, c.bsize)

	// update oldestTs as it running for quite some time, to avoid the fact that at startup
	// it only allows recent stats.
	c.oldestTs = alignedNow - int64(c.bufferLen)*c.bsize

	// Build a trace with stats which should cover 3 time buckets.
	trace := model.Trace{
		// more than 2 buckets old, should be added to the 2 bucket-old, first flush.
		testSpan(1, 0, 111, 10, "A1", "resource1", 0),
		testSpan(1, 0, 222, 3, "A1", "resource1", 0),
		// 2 buckets old, part of the first flush
		testSpan(1, 0, 24, 2, "A1", "resource1", 0),
		testSpan(2, 0, 12, 2, "A1", "resource1", 2),
		testSpan(3, 0, 40, 2, "A2", "resource2", 2),
		testSpan(4, 0, 300000000000, 2, "A2", "resource2", 2), // 5 minutes trace
		testSpan(5, 0, 30, 2, "A2", "resourcefoo", 0),
		// 1 bucket old, part of the second flush
		testSpan(6, 0, 24, 1, "A1", "resource2", 0),
		testSpan(7, 0, 12, 1, "A1", "resource1", 2),
		testSpan(8, 0, 40, 1, "A2", "resource1", 2),
		testSpan(9, 0, 30, 1, "A2", "resource2", 2),
		testSpan(10, 0, 3600000000000, 1, "A2", "resourcefoo", 0), // 1 hour trace
		// present data, part of the third flush
		testSpan(6, 0, 24, 0, "A1", "resource2", 0),
	}

	expectedCountValByKeyByTime := make(map[int64]map[string]int64)
	expectedCountValByKeyByTime[alignedNow-2*testBucketInterval] = map[string]int64{
		"query|duration|env:none,resource:resource1,service:A1":   369,
		"query|duration|env:none,resource:resource2,service:A2":   300000000040,
		"query|duration|env:none,resource:resourcefoo,service:A2": 30,
		"query|errors|env:none,resource:resource1,service:A1":     1,
		"query|errors|env:none,resource:resource2,service:A2":     2,
		"query|errors|env:none,resource:resourcefoo,service:A2":   0,
		"query|hits|env:none,resource:resource1,service:A1":       4,
		"query|hits|env:none,resource:resource2,service:A2":       2,
		"query|hits|env:none,resource:resourcefoo,service:A2":     1,
	}
	expectedCountValByKeyByTime[alignedNow-1*testBucketInterval] = map[string]int64{
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
	expectedCountValByKeyByTime[alignedNow] = map[string]int64{
		"query|duration|env:none,resource:resource2,service:A1": 24,
		"query|errors|env:none,resource:resource2,service:A1":   0,
		"query|hits|env:none,resource:resource2,service:A1":     1,
	}
	expectedCountValByKeyByTime[alignedNow+testBucketInterval] = map[string]int64{}

	trace.ComputeTopLevel()
	wt := model.NewWeightedTrace(trace, trace.GetRoot())

	testTrace := model.ProcessedTrace{
		Env:           "none",
		Trace:         trace,
		WeightedTrace: wt,
	}
	c.Add(testTrace)

	// flush every testBucketInterval
	flushTime := now
	for i := 0; i <= c.bufferLen+2; i++ {
		t.Run(fmt.Sprintf("flush-%d", i), func(t *testing.T) {
			stats := c.flushNow(flushTime)

			expectedFlushedTs := alignTs(flushTime, c.bsize) - int64(c.bufferLen)*testBucketInterval
			if len(expectedCountValByKeyByTime[expectedFlushedTs]) == 0 {
				// That's a flush for which we expect no data
				return
			}

			if !assert.Equal(1, len(stats), "We should get exactly 1 StatsBucket") {
				t.FailNow()
			}

			receivedBuckets := []model.StatsBucket{stats[0]}

			assert.Equal(expectedFlushedTs, receivedBuckets[0].Start)

			expectedCountValByKey := expectedCountValByKeyByTime[expectedFlushedTs]
			receivedCounts := receivedBuckets[0].Counts

			// verify we got all counts
			assert.Equal(len(expectedCountValByKey), len(receivedCounts), "GOT %v", receivedCounts)
			// verify values
			for key, val := range expectedCountValByKey {
				count, ok := receivedCounts[key]
				assert.True(ok, "%s was expected from concentrator", key)
				assert.Equal(val, int64(count.Value), "Wrong value for count %s", key)
			}

			// Flushing again at the same time should return nothing
			stats = c.flushNow(flushTime)

			if !assert.Equal(0, len(stats), "Second flush of the same time should be empty") {
				t.FailNow()
			}

		})
		flushTime += c.bsize
	}
}

// TestConcentratorSublayersStatsCounts tests exhaustively the sublayer stats of a single time window.
func TestConcentratorSublayersStatsCounts(t *testing.T) {
	assert := assert.New(t)
	statsChan := make(chan []model.StatsBucket)
	c := NewConcentrator([]string{}, testBucketInterval, statsChan)

	now := model.Now()
	alignedNow := now - now%c.bsize

	trace := model.Trace{
		// first bucket
		testSpan(1, 0, 2000, 0, "A1", "resource1", 0),
		testSpan(2, 1, 1000, 0, "A2", "resource2", 0),
		testSpan(3, 1, 1000, 0, "A2", "resource3", 0),
		testSpan(4, 2, 40, 0, "A3", "resource4", 0),
		testSpan(5, 4, 300, 0, "A3", "resource5", 0),
		testSpan(6, 2, 30, 0, "A3", "resource6", 0),
	}
	trace.ComputeTopLevel()
	wt := model.NewWeightedTrace(trace, trace.GetRoot())

	subtraces := trace.ExtractTopLevelSubtraces(trace.GetRoot())
	sublayers := make(map[*model.Span][]model.SublayerValue)
	for _, subtrace := range subtraces {
		subtraceSublayers := model.ComputeSublayers(subtrace.Trace)
		sublayers[subtrace.Root] = subtraceSublayers
	}

	testTrace := model.ProcessedTrace{
		Env:           "none",
		Trace:         trace,
		WeightedTrace: wt,
		Sublayers:     sublayers,
	}

	c.Add(testTrace)
	stats := c.flushNow(alignedNow + int64(c.bufferLen)*c.bsize)

	if !assert.Equal(1, len(stats), "We should get exactly 1 StatsBucket") {
		t.FailNow()
	}

	assert.Equal(alignedNow, stats[0].Start)

	var receivedCounts map[string]model.Count

	// Start with the first/older bucket
	receivedCounts = stats[0].Counts
	expectedCountValByKey := map[string]int64{
		"query|_sublayers.duration.by_service|env:none,resource:resource1,service:A1,sublayer_service:A1": 2000,
		"query|_sublayers.duration.by_service|env:none,resource:resource1,service:A1,sublayer_service:A2": 2000,
		"query|_sublayers.duration.by_service|env:none,resource:resource1,service:A1,sublayer_service:A3": 370,
		"query|_sublayers.duration.by_service|env:none,resource:resource4,service:A3,sublayer_service:A3": 340,
		"query|_sublayers.duration.by_service|env:none,resource:resource2,service:A2,sublayer_service:A2": 1000,
		"query|_sublayers.duration.by_service|env:none,resource:resource2,service:A2,sublayer_service:A3": 370,
		"query|_sublayers.duration.by_type|env:none,resource:resource1,service:A1,sublayer_type:db":       4370,
		"query|_sublayers.duration.by_type|env:none,resource:resource2,service:A2,sublayer_type:db":       1370,
		"query|_sublayers.duration.by_type|env:none,resource:resource4,service:A3,sublayer_type:db":       340,
		"query|_sublayers.span_count|env:none,resource:resource1,service:A1,:":                            6,
		"query|_sublayers.span_count|env:none,resource:resource2,service:A2,:":                            4,
		"query|_sublayers.span_count|env:none,resource:resource4,service:A3,:":                            2,
		"query|duration|env:none,resource:resource1,service:A1":                                           2000,
		"query|duration|env:none,resource:resource2,service:A2":                                           1000,
		"query|duration|env:none,resource:resource3,service:A2":                                           1000,
		"query|duration|env:none,resource:resource4,service:A3":                                           40,
		"query|duration|env:none,resource:resource6,service:A3":                                           30,
		"query|errors|env:none,resource:resource1,service:A1":                                             0,
		"query|errors|env:none,resource:resource2,service:A2":                                             0,
		"query|errors|env:none,resource:resource3,service:A2":                                             0,
		"query|errors|env:none,resource:resource4,service:A3":                                             0,
		"query|errors|env:none,resource:resource6,service:A3":                                             0,
		"query|hits|env:none,resource:resource1,service:A1":                                               1,
		"query|hits|env:none,resource:resource2,service:A2":                                               1,
		"query|hits|env:none,resource:resource3,service:A2":                                               1,
		"query|hits|env:none,resource:resource4,service:A3":                                               1,
		"query|hits|env:none,resource:resource6,service:A3":                                               1,
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

func TestConcentratorForNonTopLevelMetrics(t *testing.T) {
	assert := assert.New(t)
	statsChan := make(chan []model.StatsBucket)
	c := NewConcentrator([]string{}, testBucketInterval, statsChan)

	now := model.Now()
	alignedNow := now - now%c.bsize

	trace := model.Trace{
		testSpan(c, 1, 0, 2000, 3, "A1", "resource1", 0),
		testSpan(c, 2, 1, 1000, 3, "A2", "resource2", 0),
		testSpan(c, 4, 2, 40, 3, "A3", "resource4", 0),
		testSpan(c, 5, 4, 300, 3, "A3", "resource5", 0),
	}

	// A non-top-level span with forced metrics
	nonTopLevel := testSpan(c, 3, 2, 1000, 3, "A2", "resource3", 0)
	nonTopLevel.Name = "insert"
	nonTopLevel.Meta = map[string]string{model.TraceMetricsKey: "true"}
	trace = append(trace, nonTopLevel)

	trace.ComputeTopLevel()
	wt := model.NewWeightedTrace(trace, trace.GetRoot())
	testTrace := processedTrace{Env: "none", Trace: trace, WeightedTrace: wt}

	c.Add(testTrace)
	stats := c.Flush()

	if !assert.Equal(1, len(stats), "We should get exactly 1 StatsBucket") {
		t.FailNow()
	}

	assert.Equal(alignedNow-3*testBucketInterval, stats[0].Start)

	var receivedCounts map[string]model.Count
	receivedCounts = stats[0].Counts
	expectedCountValByKey := map[string]int64{
		"query|duration|env:none,resource:resource1,service:A1":  2000,
		"query|duration|env:none,resource:resource2,service:A2":  1000,
		"insert|duration|env:none,resource:resource3,service:A2": 1000,
		"query|duration|env:none,resource:resource4,service:A3":  40,
		"query|errors|env:none,resource:resource1,service:A1":    0,
		"query|errors|env:none,resource:resource2,service:A2":    0,
		"insert|errors|env:none,resource:resource3,service:A2":   0,
		"query|errors|env:none,resource:resource4,service:A3":    0,
		"query|hits|env:none,resource:resource1,service:A1":      1,
		"query|hits|env:none,resource:resource2,service:A2":      1,
		"insert|hits|env:none,resource:resource3,service:A2":     1,
		"query|hits|env:none,resource:resource4,service:A3":      1,
	}

	assert.Equal(len(expectedCountValByKey), len(receivedCounts))

	// verify values
	for key, val := range expectedCountValByKey {
		count, ok := receivedCounts[key]
		assert.True(ok, "%s was expected from concentrator", key)
		assert.Equal(val, int64(count.Value), "Wrong value for count %s", key)
	}
}

package writer

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
	"github.com/stretchr/testify/assert"
)

func TestStatsWriter_StatHandling(t *testing.T) {
	assert := assert.New(t)

	// Given a stats writer, its incoming channel and the endpoint that receives the payloads
	statsWriter, statsChannel, testEndpoint, _ := testStatsWriter()

	statsWriter.Start()

	// Given 2 slices of 3 test buckets
	testStats1 := []model.StatsBucket{
		fixtures.RandomStatsBucket(3),
		fixtures.RandomStatsBucket(3),
		fixtures.RandomStatsBucket(3),
	}
	testStats2 := []model.StatsBucket{
		fixtures.RandomStatsBucket(3),
		fixtures.RandomStatsBucket(3),
		fixtures.RandomStatsBucket(3),
	}

	// When sending those slices
	statsChannel <- testStats1
	statsChannel <- testStats2

	// And stopping stats writer
	close(statsChannel)
	statsWriter.Stop()

	payloads := testEndpoint.SuccessPayloads()

	// Then the endpoint should have received 2 payloads, containing all stat buckets
	assert.Len(payloads, 2, "There should be 2 payloads")

	payload1 := payloads[0]
	payload2 := payloads[1]

	expectedHeaders := map[string]string{
		"X-Datadog-Reported-Languages": strings.Join(info.Languages(), "|"),
		"Content-Type":                 "application/json",
		"Content-Encoding":             "gzip",
	}

	assertStatsPayload(assert, expectedHeaders, testStats1, &payload1)
	assertStatsPayload(assert, expectedHeaders, testStats2, &payload2)
}

func TestStatsWriter_UpdateInfoHandling(t *testing.T) {
	assert := assert.New(t)

	// Given a stats writer, its incoming channel and the endpoint that receives the payloads
	statsWriter, statsChannel, testEndpoint, statsClient := testStatsWriter()
	statsWriter.conf.UpdateInfoPeriod = 100 * time.Millisecond

	statsWriter.Start()

	expectedNumPayloads := int64(0)
	expectedNumBuckets := int64(0)
	expectedNumBytes := int64(0)
	expectedMinNumRetries := int64(0)
	expectedNumErrors := int64(0)

	// When sending 1 payload with 3 buckets
	expectedNumPayloads++
	payload1Buckets := []model.StatsBucket{
		fixtures.RandomStatsBucket(5),
		fixtures.RandomStatsBucket(5),
		fixtures.RandomStatsBucket(5),
	}
	statsChannel <- payload1Buckets
	expectedNumBuckets += 3
	expectedNumBytes += calculateStatPayloadSize(payload1Buckets)

	// And another one with another 3 buckets
	expectedNumPayloads++
	payload2Buckets := []model.StatsBucket{
		fixtures.RandomStatsBucket(5),
		fixtures.RandomStatsBucket(5),
		fixtures.RandomStatsBucket(5),
	}
	statsChannel <- payload2Buckets
	expectedNumBuckets += 3
	expectedNumBytes += calculateStatPayloadSize(payload2Buckets)

	// Wait for previous payloads to be sent
	time.Sleep(2 * statsWriter.conf.UpdateInfoPeriod)

	// And then sending a third payload with other 3 buckets with an errored out endpoint
	testEndpoint.SetError(fmt.Errorf("non retriable error"))
	expectedNumErrors++
	payload3Buckets := []model.StatsBucket{
		fixtures.RandomStatsBucket(5),
		fixtures.RandomStatsBucket(5),
		fixtures.RandomStatsBucket(5),
	}
	statsChannel <- payload3Buckets
	expectedNumBuckets += 3
	expectedNumBytes += calculateStatPayloadSize(payload3Buckets)

	// And waiting for twice the flush period to trigger payload sending and info updating
	time.Sleep(2 * statsWriter.conf.UpdateInfoPeriod)

	// And then sending a third payload with other 3 traces with an errored out endpoint with retry
	testEndpoint.SetError(&RetriableError{
		err:      fmt.Errorf("non retriable error"),
		endpoint: testEndpoint,
	})
	expectedMinNumRetries++
	payload4Buckets := []model.StatsBucket{
		fixtures.RandomStatsBucket(5),
		fixtures.RandomStatsBucket(5),
		fixtures.RandomStatsBucket(5),
	}
	statsChannel <- payload4Buckets
	expectedNumBuckets += 3
	expectedNumBytes += calculateStatPayloadSize(payload4Buckets)

	// And waiting for twice the flush period to trigger payload sending and info updating
	time.Sleep(2 * statsWriter.conf.UpdateInfoPeriod)

	close(statsChannel)
	statsWriter.Stop()

	// Then we expect some counts to have been sent to the stats client for each update tick (there should have been
	// at least 3 ticks)
	countSummaries := statsClient.GetCountSummaries()

	// Payload counts
	payloadSummary := countSummaries["datadog.trace_agent.stats_writer.payloads"]
	assert.True(len(payloadSummary.Calls) >= 3, "There should have been multiple payload count calls")
	assert.Equal(expectedNumPayloads, payloadSummary.Sum)

	// Traces counts
	bucketsSummary := countSummaries["datadog.trace_agent.stats_writer.stats_buckets"]
	assert.True(len(bucketsSummary.Calls) >= 3, "There should have been multiple stats_buckets count calls")
	assert.Equal(expectedNumBuckets, bucketsSummary.Sum)

	// Bytes counts
	bytesSummary := countSummaries["datadog.trace_agent.stats_writer.bytes"]
	assert.True(len(bytesSummary.Calls) >= 3, "There should have been multiple bytes count calls")
	assert.Equal(expectedNumBytes, bytesSummary.Sum)

	// Retry counts
	retriesSummary := countSummaries["datadog.trace_agent.stats_writer.retries"]
	assert.True(len(retriesSummary.Calls) >= 3, "There should have been multiple retries count calls")
	assert.True(retriesSummary.Sum >= expectedMinNumRetries)

	// Error counts
	errorsSummary := countSummaries["datadog.trace_agent.stats_writer.errors"]
	assert.True(len(errorsSummary.Calls) >= 3, "There should have been multiple errors count calls")
	assert.Equal(expectedNumErrors, errorsSummary.Sum)
}

func calculateStatPayloadSize(buckets []model.StatsBucket) int64 {
	statsPayload := &model.StatsPayload{
		HostName: testHostName,
		Env:      testEnv,
		Stats:    buckets,
	}

	data, _ := model.EncodeStatsPayload(statsPayload)
	return int64(len(data))
}

func assertStatsPayload(assert *assert.Assertions, headers map[string]string, buckets []model.StatsBucket,
	payload *Payload) {
	statsPayload := model.StatsPayload{}

	reader := bytes.NewBuffer(payload.Bytes)
	gzipReader, err := gzip.NewReader(reader)

	assert.NoError(err, "Gzip reader should work correctly")

	jsonDecoder := json.NewDecoder(gzipReader)

	assert.NoError(jsonDecoder.Decode(&statsPayload), "Stats payload should unmarshal correctly")

	assert.Equal(headers, payload.Headers, "Headers should match expectation")
	assert.Equal(testHostName, statsPayload.HostName, "Hostname should match expectation")
	assert.Equal(testEnv, statsPayload.Env, "Env should match expectation")
	assert.Equal(buckets, statsPayload.Stats, "Stat buckets should match expectation")
}

func testStatsWriter() (*StatsWriter, chan []model.StatsBucket, *testEndpoint, *fixtures.TestStatsClient) {
	statsChannel := make(chan []model.StatsBucket)
	conf := &config.AgentConfig{
		HostName:          testHostName,
		DefaultEnv:        testEnv,
		StatsWriterConfig: writerconfig.DefaultStatsWriterConfig(),
	}
	statsWriter := NewStatsWriter(conf, statsChannel)
	testEndpoint := &testEndpoint{}
	statsWriter.BaseWriter.payloadSender.setEndpoint(testEndpoint)
	testStatsClient := &fixtures.TestStatsClient{}
	statsWriter.statsClient = testStatsClient

	return statsWriter, statsChannel, testEndpoint, testStatsClient
}

package writer

import (
	"fmt"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/backoff"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
	"github.com/stretchr/testify/assert"
)

func TestNewPayloadSetsCreationDate(t *testing.T) {
	assert := assert.New(t)

	newPayload := NewPayload(nil, nil)

	assert.WithinDuration(time.Now(), newPayload.CreationDate, 1*time.Second)
}

func TestQueuablePayloadSender_WorkingEndpoint(t *testing.T) {
	assert := assert.New(t)

	// Given an endpoint that doesn't fail
	workingEndpoint := &TestEndpoint{}

	// And a queuable sender using that endpoint
	queuableSender := NewQueuablePayloadSender(workingEndpoint)

	// And a test monitor for that sender
	monitor := NewTestPayloadSenderMonitor(queuableSender)

	// When we start the sender
	monitor.Start()
	queuableSender.Start()

	// And send some payloads
	payload1 := RandomPayload()
	queuableSender.Send(payload1)
	payload2 := RandomPayload()
	queuableSender.Send(payload2)
	payload3 := RandomPayload()
	queuableSender.Send(payload3)
	payload4 := RandomPayload()
	queuableSender.Send(payload4)
	payload5 := RandomPayload()
	queuableSender.Send(payload5)

	// And stop the sender
	queuableSender.Stop()
	monitor.Stop()

	// Then we expect all sent payloads to have been successfully sent
	successPayloads := monitor.SuccessPayloads()
	errorPayloads := monitor.FailurePayloads()
	assert.Equal([]Payload{*payload1, *payload2, *payload3, *payload4, *payload5}, successPayloads,
		"Expect all sent payloads to have been successful")
	assert.Equal(successPayloads, workingEndpoint.SuccessPayloads, "Expect sender and endpoint to match on successful payloads")
	assert.Len(errorPayloads, 0, "No payloads should have errored out on send")
	assert.Len(workingEndpoint.ErrorPayloads, 0, "No payloads should have errored out on send")
}

func TestQueuablePayloadSender_FlakyEndpoint(t *testing.T) {
	assert := assert.New(t)

	// Given an endpoint that initially works ok
	flakyEndpoint := &TestEndpoint{}

	// And a test backoff timer that can be triggered on-demand
	testBackoffTimer := backoff.NewTestBackoffTimer()

	// And a queuable sender using said endpoint and timer
	conf := writerconfig.DefaultQueuablePayloadSenderConf()
	queuableSender := NewCustomQueuablePayloadSender(flakyEndpoint, conf)
	queuableSender.backoffTimer = testBackoffTimer
	syncBarrier := make(chan interface{})
	queuableSender.syncBarrier = syncBarrier

	// And a test monitor for that sender
	monitor := NewTestPayloadSenderMonitor(queuableSender)

	monitor.Start()
	queuableSender.Start()

	// With a working endpoint
	// We send some payloads
	payload1 := RandomPayload()
	queuableSender.Send(payload1)
	payload2 := RandomPayload()
	queuableSender.Send(payload2)

	// Make sure sender processed both payloads
	syncBarrier <- nil

	assert.Equal(0, queuableSender.NumQueuedPayloads(), "Expect no queued payloads")

	// With a failing endpoint with a retriable error
	flakyEndpoint.Err = &RetriableError{err: fmt.Errorf("bleh"), endpoint: flakyEndpoint}
	// We send some payloads
	payload3 := RandomPayload()
	queuableSender.Send(payload3)
	payload4 := RandomPayload()
	queuableSender.Send(payload4)
	// And retry once
	testBackoffTimer.TriggerTick()
	// And retry twice
	testBackoffTimer.TriggerTick()

	// Make sure sender processed both ticks
	syncBarrier <- nil

	assert.Equal(2, queuableSender.NumQueuedPayloads(), "Expect 2 queued payloads")

	// With the previously failing endpoint working again
	flakyEndpoint.Err = nil
	// We retry for the third time
	testBackoffTimer.TriggerTick()

	// Make sure sender processed previous tick
	syncBarrier <- nil

	assert.Equal(0, queuableSender.NumQueuedPayloads(), "Expect no queued payloads")

	// Finally, with a failing endpoint with a non-retriable error
	flakyEndpoint.Err = fmt.Errorf("non retriable bleh")
	// We send some payloads
	payload5 := RandomPayload()
	queuableSender.Send(payload5)
	payload6 := RandomPayload()
	queuableSender.Send(payload6)

	// Make sure sender processed previous payloads
	syncBarrier <- nil

	assert.Equal(0, queuableSender.NumQueuedPayloads(), "Expect no queued payloads")

	// With the previously failing endpoint working again
	flakyEndpoint.Err = nil
	// We retry just in case there's something in the queue
	testBackoffTimer.TriggerTick()

	// And stop the sender
	queuableSender.Stop()
	monitor.Stop()

	// Then we expect payloads sent during working endpoint or those that were retried due to retriable errors to have
	// been sent eventually (and in order). Those that failed because of non-retriable errors should have been discarded
	// even after a retry.
	successPayloads := monitor.SuccessPayloads()
	errorPayloads := monitor.FailurePayloads()
	retryPayloads := monitor.RetryPayloads()
	assert.Equal([]Payload{*payload1, *payload2, *payload3, *payload4}, successPayloads,
		"Expect all sent payloads to have been successful")
	assert.Equal(successPayloads, flakyEndpoint.SuccessPayloads, "Expect sender and endpoint to match on successful payloads")
	// Expect 3 retry events for payload 3 (one because of first send, two others because of the two retries)
	assert.Equal([]Payload{*payload3, *payload3, *payload3}, retryPayloads, "Expect payload 3 to have been retries 3 times")
	// We expect payloads 5 and 6 to appear in error payloads as they failed for non-retriable errors.
	assert.Equal([]Payload{*payload5, *payload6}, errorPayloads, "Expect errored payloads to have been discarded as expected")
}

func TestQueuablePayloadSender_MaxQueuedPayloads(t *testing.T) {
	assert := assert.New(t)

	// Given an endpoint that continuously throws out retriable errors
	flakyEndpoint := &TestEndpoint{}
	flakyEndpoint.Err = &RetriableError{err: fmt.Errorf("bleh"), endpoint: flakyEndpoint}

	// And a test backoff timer that can be triggered on-demand
	testBackoffTimer := backoff.NewTestBackoffTimer()

	// And a queuable sender using said endpoint and timer and with a meager max queued payloads value of 1
	conf := writerconfig.DefaultQueuablePayloadSenderConf()
	conf.MaxQueuedPayloads = 1
	queuableSender := NewCustomQueuablePayloadSender(flakyEndpoint, conf)
	queuableSender.backoffTimer = testBackoffTimer
	syncBarrier := make(chan interface{})
	queuableSender.syncBarrier = syncBarrier

	// And a test monitor for that sender
	monitor := NewTestPayloadSenderMonitor(queuableSender)

	monitor.Start()
	queuableSender.Start()

	// When sending a first payload
	payload1 := RandomPayload()
	queuableSender.Send(payload1)

	// Followed by another one
	payload2 := RandomPayload()
	queuableSender.Send(payload2)

	// Followed by a third
	payload3 := RandomPayload()
	queuableSender.Send(payload3)

	// Ensure previous payloads were processed
	syncBarrier <- nil

	// Then, when the endpoint finally works
	flakyEndpoint.Err = nil

	// And we trigger a retry
	testBackoffTimer.TriggerTick()

	// Ensure tick was processed
	syncBarrier <- nil

	// Then we should have no queued payloads
	assert.Equal(0, queuableSender.NumQueuedPayloads(), "We should have no queued payloads")

	// When we stop the sender
	queuableSender.Stop()
	monitor.Stop()

	// Then endpoint should have received only payload3. Other should have been discarded because max queued payloads
	// is 1
	assert.Equal([]Payload{*payload3}, flakyEndpoint.SuccessPayloads, "Endpoint should have received only payload 3")

	// Monitor should agree on previous fact
	assert.Equal([]Payload{*payload3}, monitor.SuccessPayloads(),
		"Monitor should agree with endpoint on succesful payloads")
	assert.Equal([]Payload{*payload1, *payload2}, monitor.FailurePayloads(),
		"Monitor should agree with endpoint on failed payloads")
	assert.Contains(monitor.FailureEvents[0].Error.Error(), "max queued payloads",
		"Monitor failure event should mention correct reason for error")
	assert.Contains(monitor.FailureEvents[1].Error.Error(), "max queued payloads",
		"Monitor failure event should mention correct reason for error")
}

func TestQueuablePayloadSender_MaxQueuedBytes(t *testing.T) {
	assert := assert.New(t)

	// Given an endpoint that continuously throws out retriable errors
	flakyEndpoint := &TestEndpoint{}
	flakyEndpoint.Err = &RetriableError{err: fmt.Errorf("bleh"), endpoint: flakyEndpoint}

	// And a test backoff timer that can be triggered on-demand
	testBackoffTimer := backoff.NewTestBackoffTimer()

	// And a queuable sender using said endpoint and timer and with a meager max size of 10 bytes
	conf := writerconfig.DefaultQueuablePayloadSenderConf()
	conf.MaxQueuedBytes = 10
	queuableSender := NewCustomQueuablePayloadSender(flakyEndpoint, conf)
	queuableSender.backoffTimer = testBackoffTimer
	syncBarrier := make(chan interface{})
	queuableSender.syncBarrier = syncBarrier

	// And a test monitor for that sender
	monitor := NewTestPayloadSenderMonitor(queuableSender)

	monitor.Start()
	queuableSender.Start()

	// When sending a first payload of 4 bytes
	payload1 := RandomSizedPayload(4)
	queuableSender.Send(payload1)

	// Followed by another one of 2 bytes
	payload2 := RandomSizedPayload(2)
	queuableSender.Send(payload2)

	// Followed by a third of 8 bytes
	payload3 := RandomSizedPayload(8)
	queuableSender.Send(payload3)

	// Ensure previous payloads were processed
	syncBarrier <- nil

	// Then, when the endpoint finally works
	flakyEndpoint.Err = nil

	// And we trigger a retry
	testBackoffTimer.TriggerTick()

	// Ensure tick was processed
	syncBarrier <- nil

	// Then we should have no queued payloads
	assert.Equal(0, queuableSender.NumQueuedPayloads(), "We should have no queued payloads")

	// When we stop the sender
	queuableSender.Stop()
	monitor.Stop()

	// Then endpoint should have received payload2 and payload3. Payload1 should have been discarded because keeping all
	// 3 would have put us over the max size of sender
	assert.Equal([]Payload{*payload2, *payload3}, flakyEndpoint.SuccessPayloads,
		"Endpoint should have received only payload 2 and 3 (in that order)")

	// Monitor should agree on previous fact
	assert.Equal([]Payload{*payload2, *payload3}, monitor.SuccessPayloads(),
		"Monitor should agree with endpoint on succesful payloads")
	assert.Equal([]Payload{*payload1}, monitor.FailurePayloads(),
		"Monitor should agree with endpoint on failed payloads")
	assert.Contains(monitor.FailureEvents[0].Error.Error(), "max queued bytes",
		"Monitor failure event should mention correct reason for error")
}

func TestQueuablePayloadSender_DropBigPayloadsOnRetry(t *testing.T) {
	assert := assert.New(t)

	// Given an endpoint that continuously throws out retriable errors
	flakyEndpoint := &TestEndpoint{}
	flakyEndpoint.Err = &RetriableError{err: fmt.Errorf("bleh"), endpoint: flakyEndpoint}

	// And a test backoff timer that can be triggered on-demand
	testBackoffTimer := backoff.NewTestBackoffTimer()

	// And a queuable sender using said endpoint and timer and with a meager max size of 10 bytes
	conf := writerconfig.DefaultQueuablePayloadSenderConf()
	conf.MaxQueuedBytes = 10
	queuableSender := NewCustomQueuablePayloadSender(flakyEndpoint, conf)
	queuableSender.backoffTimer = testBackoffTimer
	syncBarrier := make(chan interface{})
	queuableSender.syncBarrier = syncBarrier

	// And a test monitor for that sender
	monitor := NewTestPayloadSenderMonitor(queuableSender)

	monitor.Start()
	queuableSender.Start()

	// When sending a payload of 12 bytes
	payload1 := RandomSizedPayload(12)
	queuableSender.Send(payload1)

	// Ensure previous payloads were processed
	syncBarrier <- nil

	// Then, when the endpoint finally works
	flakyEndpoint.Err = nil

	// And we trigger a retry
	testBackoffTimer.TriggerTick()

	// Ensure tick was processed
	syncBarrier <- nil

	// Then we should have no queued payloads
	assert.Equal(0, queuableSender.NumQueuedPayloads(), "We should have no queued payloads")

	// When we stop the sender
	queuableSender.Stop()
	monitor.Stop()

	// Then endpoint should have received no payloads because payload1 was too big to store in queue.
	assert.Len(flakyEndpoint.SuccessPayloads, 0, "Endpoint should have received no payloads")

	// And monitor should have received failed event for payload1 with correct reason
	assert.Equal([]Payload{*payload1}, monitor.FailurePayloads(),
		"Monitor should agree with endpoint on failed payloads")
	assert.Contains(monitor.FailureEvents[0].Error.Error(), "bigger than max size",
		"Monitor failure event should mention correct reason for error")
}

func TestQueuablePayloadSender_SendBigPayloadsIfNoRetry(t *testing.T) {
	assert := assert.New(t)

	// Given an endpoint that works
	workingEndpoint := &TestEndpoint{}

	// And a test backoff timer that can be triggered on-demand
	testBackoffTimer := backoff.NewTestBackoffTimer()

	// And a queuable sender using said endpoint and timer and with a meager max size of 10 bytes
	conf := writerconfig.DefaultQueuablePayloadSenderConf()
	conf.MaxQueuedBytes = 10
	queuableSender := NewCustomQueuablePayloadSender(workingEndpoint, conf)
	queuableSender.backoffTimer = testBackoffTimer
	syncBarrier := make(chan interface{})
	queuableSender.syncBarrier = syncBarrier

	// And a test monitor for that sender
	monitor := NewTestPayloadSenderMonitor(queuableSender)

	monitor.Start()
	queuableSender.Start()

	// When sending a payload of 12 bytes
	payload1 := RandomSizedPayload(12)
	queuableSender.Send(payload1)

	// Ensure previous payloads were processed
	syncBarrier <- nil

	// Then we should have no queued payloads
	assert.Equal(0, queuableSender.NumQueuedPayloads(), "We should have no queued payloads")

	// When we stop the sender
	queuableSender.Stop()
	monitor.Stop()

	// Then endpoint should have received payload1 because although it was big, it didn't get queued.
	assert.Equal([]Payload{*payload1}, workingEndpoint.SuccessPayloads, "Endpoint should have received payload1")

	// And monitor should have received success event for payload1
	assert.Equal([]Payload{*payload1}, monitor.SuccessPayloads(),
		"Monitor should agree with endpoint on success payloads")
}

func TestQueuablePayloadSender_MaxAge(t *testing.T) {
	assert := assert.New(t)

	// Given an endpoint that continuously throws out retriable errors
	flakyEndpoint := &TestEndpoint{}
	flakyEndpoint.Err = &RetriableError{err: fmt.Errorf("bleh"), endpoint: flakyEndpoint}

	// And a test backoff timer that can be triggered on-demand
	testBackoffTimer := backoff.NewTestBackoffTimer()

	// And a queuable sender using said endpoint and timer and with a meager max age of 100ms
	conf := writerconfig.DefaultQueuablePayloadSenderConf()
	conf.MaxAge = 100 * time.Millisecond
	queuableSender := NewCustomQueuablePayloadSender(flakyEndpoint, conf)
	queuableSender.backoffTimer = testBackoffTimer
	syncBarrier := make(chan interface{})
	queuableSender.syncBarrier = syncBarrier

	// And a test monitor for that sender
	monitor := NewTestPayloadSenderMonitor(queuableSender)

	monitor.Start()
	queuableSender.Start()

	// When sending two payloads one after the other
	payload1 := RandomPayload()
	queuableSender.Send(payload1)
	payload2 := RandomPayload()
	queuableSender.Send(payload2)

	// And then sleeping for 500ms
	time.Sleep(500 * time.Millisecond)

	// And then sending a third payload
	payload3 := RandomPayload()
	queuableSender.Send(payload3)

	// And then triggering a retry
	testBackoffTimer.TriggerTick()

	// Ensure tick was processed
	syncBarrier <- nil

	// Then, when the endpoint finally works
	flakyEndpoint.Err = nil

	// And we trigger a retry
	testBackoffTimer.TriggerTick()

	// Ensure tick was processed
	syncBarrier <- nil

	// Then we should have no queued payloads
	assert.Equal(0, queuableSender.NumQueuedPayloads(), "We should have no queued payloads")

	// When we stop the sender
	queuableSender.Stop()
	monitor.Stop()

	// Then endpoint should have received only payload3. Because payload1 and payload2 were too old after the failed
	// retry (first TriggerTick).
	assert.Equal([]Payload{*payload3}, flakyEndpoint.SuccessPayloads, "Endpoint should have received only payload 3")

	// And monitor should have received failed events for payload1 and payload2 with correct reason
	assert.Equal([]Payload{*payload1, *payload2}, monitor.FailurePayloads(),
		"Monitor should agree with endpoint on failed payloads")
	assert.Contains(monitor.FailureEvents[0].Error.Error(), "older than max age",
		"Monitor failure event should mention correct reason for error")
}

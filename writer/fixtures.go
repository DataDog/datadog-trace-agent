package writer

import (
	"math/rand"
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/fixtures"
)

// PayloadConstructedHandlerArgs encodes the arguments passed to a PayloadConstructedHandler call.
type PayloadConstructedHandlerArgs struct {
	payload *Payload
	stats   interface{}
}

// TestEndpoint represents a mocked endpoint that replies with a configurable error and records successful and failed
// payloads.
type TestEndpoint struct {
	Err             error
	SuccessPayloads []Payload
	ErrorPayloads   []Payload
}

// Write mocks the writing of a payload to a remote endpoint, recording it and replying with the configured error (or
// success in its absence).
func (e *TestEndpoint) Write(payload *Payload) error {
	if e.Err != nil {
		e.ErrorPayloads = append(e.ErrorPayloads, *payload)
	} else {
		e.SuccessPayloads = append(e.SuccessPayloads, *payload)
	}

	return e.Err
}

func (e *TestEndpoint) String() string {
	return "TestEndpoint"
}

// RandomPayload creates a new payload instance using random data and up to 32 bytes.
func RandomPayload() *Payload {
	return RandomSizedPayload(rand.Intn(32))
}

// RandomSizedPayload creates a new payload instance using random data with the specified size.
func RandomSizedPayload(size int) *Payload {
	return NewPayload(fixtures.RandomSizedBytes(size), fixtures.RandomStringMap())
}

// TestPayloadSender is a PayloadSender that is connected to a TestEndpoint, used for testing.
type TestPayloadSender struct {
	testEndpoint *TestEndpoint
	BasePayloadSender
}

// NewTestPayloadSender creates a new instance of a TestPayloadSender.
func NewTestPayloadSender() *TestPayloadSender {
	testEndpoint := &TestEndpoint{}
	return &TestPayloadSender{
		testEndpoint:      testEndpoint,
		BasePayloadSender: *NewBasePayloadSender(testEndpoint),
	}
}

// Start asynchronously starts this payload sender.
func (c *TestPayloadSender) Start() {
	go c.Run()
}

// Run executes the core loop of this sender.
func (c *TestPayloadSender) Run() {
	c.exitWG.Add(1)
	defer c.exitWG.Done()

	for {
		select {
		case payload := <-c.in:
			stats, err := c.send(payload)

			if err != nil {
				c.notifyError(payload, err, stats)
			} else {
				c.notifySuccess(payload, stats)
			}
		case <-c.exit:
			return
		}
	}
}

// Payloads allows access to all payloads recorded as being successfully sent by this sender.
func (c *TestPayloadSender) Payloads() []Payload {
	return c.testEndpoint.SuccessPayloads
}

// Endpoint allows access to the underlying TestEndpoint.
func (c *TestPayloadSender) Endpoint() *TestEndpoint {
	return c.testEndpoint
}

func (c *TestPayloadSender) setEndpoint(endpoint Endpoint) {
	c.testEndpoint = endpoint.(*TestEndpoint)
}

// TestPayloadSenderMonitor monitors a PayloadSender and stores all events
type TestPayloadSenderMonitor struct {
	SuccessEvents []SenderSuccessEvent
	FailureEvents []SenderFailureEvent
	RetryEvents   []SenderRetryEvent

	sender PayloadSender

	exit   chan struct{}
	exitWG sync.WaitGroup
}

// NewTestPayloadSenderMonitor creates a new TestPayloadSenderMonitor monitoring the specified sender.
func NewTestPayloadSenderMonitor(sender PayloadSender) *TestPayloadSenderMonitor {
	return &TestPayloadSenderMonitor{
		sender: sender,
		exit:   make(chan struct{}),
	}
}

// Start asynchronously starts this payload monitor.
func (m *TestPayloadSenderMonitor) Start() {
	go m.Run()
}

// Run executes the core loop of this monitor.
func (m *TestPayloadSenderMonitor) Run() {
	m.exitWG.Add(1)
	defer m.exitWG.Done()

	for {
		select {
		case event := <-m.sender.Monitor():
			if event == nil {
				continue
			}

			switch event := event.(type) {
			case SenderSuccessEvent:
				m.SuccessEvents = append(m.SuccessEvents, event)
			case SenderFailureEvent:
				m.FailureEvents = append(m.FailureEvents, event)
			case SenderRetryEvent:
				m.RetryEvents = append(m.RetryEvents, event)
			default:
				log.Errorf("Unknown event of type %T", event)
			}
		case <-m.exit:
			return
		}
	}
}

// Stop stops this payload monitor and waits for it to stop.
func (m *TestPayloadSenderMonitor) Stop() {
	close(m.exit)
	m.exitWG.Wait()
}

// SuccessPayloads returns a slice containing all successful payloads.
func (m *TestPayloadSenderMonitor) SuccessPayloads() []Payload {
	result := make([]Payload, len(m.SuccessEvents))

	for i, successEvent := range m.SuccessEvents {
		result[i] = *successEvent.Payload
	}

	return result
}

// FailurePayloads returns a slice containing all failed payloads.
func (m *TestPayloadSenderMonitor) FailurePayloads() []Payload {
	result := make([]Payload, len(m.FailureEvents))

	for i, successEvent := range m.FailureEvents {
		result[i] = *successEvent.Payload
	}

	return result
}

// RetryPayloads returns a slice containing all failed payloads.
func (m *TestPayloadSenderMonitor) RetryPayloads() []Payload {
	result := make([]Payload, len(m.RetryEvents))

	for i, successEvent := range m.RetryEvents {
		result[i] = *successEvent.Payload
	}

	return result
}

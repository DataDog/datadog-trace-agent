package writer

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/backoff"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
)

// Payload represents a data payload to be sent to some endpoint
type Payload struct {
	CreationDate time.Time
	Bytes        []byte
	Headers      map[string]string
}

// NewPayload constructs a new payload object with the provided data and with CreationDate initialized to the current
// time.
func NewPayload(bytes []byte, headers map[string]string) *Payload {
	return &Payload{
		CreationDate: time.Now(),
		Bytes:        bytes,
		Headers:      headers,
	}
}

// SendStats represents basic stats related to the sending of a payload.
type SendStats struct {
	SendTime time.Duration
}

// SenderSuccessEvent encodes information related to the successful sending of a payload.
type SenderSuccessEvent struct {
	Payload   *Payload
	SendStats SendStats
}

// SenderFailureEvent encodes information related to the failed sending of a payload without subsequent retry.
type SenderFailureEvent struct {
	Payload   *Payload
	SendStats SendStats
	Error     error
}

// SenderRetryEvent encodes information related to the failed sending of a payload that triggered an automatic retry.
type SenderRetryEvent struct {
	Payload    *Payload
	SendTime   time.Duration
	Error      error
	RetryDelay time.Duration
	RetryNum   int
}

// PayloadSender represents an object capable of asynchronously sending payloads to some endpoint.
type PayloadSender interface {
	Start()
	Run()
	Stop()
	Send(payload *Payload)
	setEndpoint(endpoint Endpoint)
	Monitor() <-chan interface{}
}

// BasePayloadSender encodes structures and behaviours common to most PayloadSenders.
type BasePayloadSender struct {
	in       chan *Payload
	monitor  chan interface{}
	endpoint Endpoint

	exit   chan struct{}
	exitWG sync.WaitGroup
}

// NewBasePayloadSender creates a new instance of a BasePayloadSender using the provided endpoint.
func NewBasePayloadSender(endpoint Endpoint) *BasePayloadSender {
	return &BasePayloadSender{
		in:       make(chan *Payload),
		monitor:  make(chan interface{}),
		endpoint: endpoint,
		exit:     make(chan struct{}),
	}
}

// Send sends a single isolated payload through this sender.
func (s *BasePayloadSender) Send(payload *Payload) {
	s.in <- payload
}

// Stop asks this sender to stop and waits until it correctly stops.
func (s *BasePayloadSender) Stop() {
	close(s.exit)
	s.exitWG.Wait()
	close(s.in)
	close(s.monitor)
}

func (s *BasePayloadSender) setEndpoint(endpoint Endpoint) {
	s.endpoint = endpoint
}

// Monitor allows an external entity to monitor events of this sender by receiving Sender*Event structs.
func (s *BasePayloadSender) Monitor() <-chan interface{} {
	return s.monitor
}

// send will send the provided payload without any checks.
func (s *BasePayloadSender) send(payload *Payload) (SendStats, error) {
	if payload == nil {
		return SendStats{}, nil
	}

	startFlush := time.Now()
	err := s.endpoint.Write(payload)

	sendStats := SendStats{
		SendTime: time.Since(startFlush),
	}

	return sendStats, err
}

func (s *BasePayloadSender) notifySuccess(payload *Payload, sendStats SendStats) {
	s.monitor <- SenderSuccessEvent{
		Payload:   payload,
		SendStats: sendStats,
	}
}

func (s *BasePayloadSender) notifyError(payload *Payload, err error, sendStats SendStats) {
	s.monitor <- SenderFailureEvent{
		Payload:   payload,
		SendStats: sendStats,
		Error:     err,
	}
}

func (s *BasePayloadSender) notifyRetry(payload *Payload, err error, delay time.Duration, retryNum int) {
	s.monitor <- SenderRetryEvent{
		Payload:    payload,
		Error:      err,
		RetryDelay: delay,
		RetryNum:   retryNum,
	}
}

// QueuablePayloadSender is a specific implementation of a PayloadSender that will queue new payloads on error and
// retry sending them according to some configurable BackoffTimer.
type QueuablePayloadSender struct {
	conf              writerconfig.QueuablePayloadSenderConf
	queuedPayloads    *list.List
	queuing           bool
	currentQueuedSize int64

	backoffTimer backoff.Timer

	// Test helper
	syncBarrier <-chan interface{}

	BasePayloadSender
}

// NewQueuablePayloadSender constructs a new QueuablePayloadSender with default configuration to send payloads to the
// provided endpoint.
func NewQueuablePayloadSender(endpoint Endpoint) *QueuablePayloadSender {
	return NewCustomQueuablePayloadSender(endpoint, writerconfig.DefaultQueuablePayloadSenderConf())
}

// NewCustomQueuablePayloadSender constructs a new QueuablePayloadSender with custom configuration to send payloads to
// the provided endpoint.
func NewCustomQueuablePayloadSender(endpoint Endpoint, conf writerconfig.QueuablePayloadSenderConf) *QueuablePayloadSender {
	return &QueuablePayloadSender{
		conf:              conf,
		queuedPayloads:    list.New(),
		backoffTimer:      backoff.NewCustomExponentialTimer(conf.ExponentialBackoff),
		BasePayloadSender: *NewBasePayloadSender(endpoint),
	}
}

// Start asynchronously starts this QueueablePayloadSender.
func (s *QueuablePayloadSender) Start() {
	go func() {
		defer watchdog.LogOnPanic()
		s.Run()
	}()
}

// Run executes the QueuablePayloadSender main logic synchronously.
func (s *QueuablePayloadSender) Run() {
	s.exitWG.Add(1)
	defer s.exitWG.Done()

	for {
		select {
		case payload := <-s.in:
			if stats, err := s.sendOrQueue(payload); err != nil {
				log.Debugf("Error while sending or queueing payload. err=%v", err)
				s.notifyError(payload, err, stats)
			}
		case <-s.backoffTimer.ReceiveTick():
			s.flushQueue()
		case <-s.syncBarrier:
			// TODO: Is there a way of avoiding this? I want Promises in Go :(((
			// This serves as a barrier (assuming syncBarrier is an unbuffered channel). Used for testing
			continue
		case <-s.exit:
			log.Info("exiting payload sender, try flushing whatever is left")
			s.flushQueue()
			return
		}
	}
}

// NumQueuedPayloads returns the number of payloads currently waiting in the queue for a retry
func (s *QueuablePayloadSender) NumQueuedPayloads() int {
	return s.queuedPayloads.Len()
}

// sendOrQueue sends the provided payload or queues it if this sender is currently queueing payloads.
func (s *QueuablePayloadSender) sendOrQueue(payload *Payload) (SendStats, error) {
	stats := SendStats{}

	if payload == nil {
		return stats, nil
	}

	var err error

	if !s.queuing {
		if stats, err = s.send(payload); err != nil {
			if _, ok := err.(*RetriableError); ok {
				// If error is retriable, start a queue and schedule a retry
				retryNum, delay := s.backoffTimer.ScheduleRetry(err)
				log.Debugf("Got retriable error. Starting a queue. delay=%s, err=%v", delay, err)
				s.notifyRetry(payload, err, delay, retryNum)
				return stats, s.enqueue(payload)
			}
		} else {
			// If success, notify
			log.Tracef("Successfully sent direct payload: %v", payload)
			s.notifySuccess(payload, stats)
		}
	} else {
		return stats, s.enqueue(payload)
	}

	return stats, err
}

func (s *QueuablePayloadSender) enqueue(payload *Payload) error {
	if !s.queuing {
		s.queuing = true
	}

	// Start by discarding payloads that are too old, freeing up memory
	s.discardOldPayloads()

	for s.conf.MaxQueuedPayloads > 0 && s.queuedPayloads.Len() >= s.conf.MaxQueuedPayloads {
		log.Debugf("Dropping existing payload because max queued payloads reached: %d", s.conf.MaxQueuedPayloads)
		if _, err := s.dropOldestPayload("max queued payloads reached"); err != nil {
			panic(fmt.Errorf("unable to respect max queued payloads value of %d", s.conf.MaxQueuedPayloads))
		}
	}

	newPayloadSize := int64(len(payload.Bytes))

	if s.conf.MaxQueuedBytes > 0 && newPayloadSize > s.conf.MaxQueuedBytes {
		log.Debugf("Payload bigger than max size: size=%d, max size=%d", newPayloadSize, s.conf.MaxQueuedBytes)
		return fmt.Errorf("unable to queue payload bigger than max size: payload size=%d, max size=%d",
			newPayloadSize, s.conf.MaxQueuedBytes)
	}

	for s.conf.MaxQueuedBytes > 0 && s.currentQueuedSize+newPayloadSize > s.conf.MaxQueuedBytes {
		if _, err := s.dropOldestPayload("max queued bytes reached"); err != nil {
			// Should never happen because we know we can fit it in
			panic(fmt.Errorf("unable to find space for queueing payload of size %d: %v", newPayloadSize, err))
		}
	}

	log.Tracef("Queuing new payload: %v", payload)
	s.queuedPayloads.PushBack(payload)
	s.currentQueuedSize += newPayloadSize

	return nil
}

func (s *QueuablePayloadSender) flushQueue() error {
	log.Debugf("Attempting to flush queue with %d payloads", s.NumQueuedPayloads())

	// Start by discarding payloads that are too old
	s.discardOldPayloads()

	// For the remaining ones, try to send them one by one
	var next *list.Element
	for e := s.queuedPayloads.Front(); e != nil; e = next {
		payload := e.Value.(*Payload)

		var err error
		var stats SendStats

		if stats, err = s.send(payload); err != nil {
			if _, ok := err.(*RetriableError); ok {
				// If send failed due to a retriable error, retry flush later
				retryNum, delay := s.backoffTimer.ScheduleRetry(err)
				log.Debugf("Got retriable error. Retrying flush later: retry=%d, delay=%s, err=%v",
					retryNum, delay, err)
				s.notifyRetry(payload, err, delay, retryNum)
				// Don't try to send following. We'll flush all later.
				return err
			}

			// If send failed due to non-retriable error, notify error and drop it
			log.Debugf("Dropping payload due to non-retriable error: err=%v, payload=%v", err, payload)
			s.notifyError(payload, err, stats)
			next = s.removeQueuedPayload(e)
			// Try sending next ones
			continue
		}

		// If successful, remove payload from queue
		log.Tracef("Successfully sent a queued payload: %v", payload)
		s.notifySuccess(payload, stats)
		next = s.removeQueuedPayload(e)
	}

	s.queuing = false
	s.backoffTimer.Reset()

	return nil
}

func (s *QueuablePayloadSender) removeQueuedPayload(e *list.Element) *list.Element {
	next := e.Next()
	payload := e.Value.(*Payload)
	s.currentQueuedSize -= int64(len(payload.Bytes))
	s.queuedPayloads.Remove(e)
	return next
}

// Discard those payloads that are older than max age.
func (s *QueuablePayloadSender) discardOldPayloads() {
	// If MaxAge <= 0 then age limitation is disabled so do nothing
	if s.conf.MaxAge <= 0 {
		return
	}

	var next *list.Element

	for e := s.queuedPayloads.Front(); e != nil; e = next {
		payload := e.Value.(*Payload)

		age := time.Since(payload.CreationDate)

		// Payloads are kept in order so as soon as we find one that isn't, we can break out
		if age < s.conf.MaxAge {
			break
		}

		err := fmt.Errorf("payload is older than max age: age=%v, max age=%v", age, s.conf.MaxAge)
		log.Tracef("Discarding payload: err=%v, payload=%v", err, payload)
		s.notifyError(payload, err, SendStats{})
		next = s.removeQueuedPayload(e)
	}
}

// Payloads are kept in order so dropping the one at the front guarantees we're dropping the oldest
func (s *QueuablePayloadSender) dropOldestPayload(reason string) (*Payload, error) {
	if s.queuedPayloads.Len() == 0 {
		return nil, fmt.Errorf("no queued payloads")
	}

	err := fmt.Errorf("payload dropped: %s", reason)
	droppedPayload := s.queuedPayloads.Front().Value.(*Payload)
	s.removeQueuedPayload(s.queuedPayloads.Front())
	s.notifyError(droppedPayload, err, SendStats{})

	return droppedPayload, nil
}

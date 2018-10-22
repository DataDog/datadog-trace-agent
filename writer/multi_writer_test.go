package writer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/writer/config"
	"github.com/stretchr/testify/assert"
)

func TestNewMultiSenderFactory(t *testing.T) {
	cfg := config.DefaultQueuablePayloadSenderConf()

	t.Run("one", func(t *testing.T) {
		endpoint := &DatadogEndpoint{Host: "host1", APIKey: "key1"}
		sender, ok := newMultiSender([]Endpoint{endpoint}, cfg).(*QueuablePayloadSender)
		assert := assert.New(t)
		assert.True(ok)
		assert.EqualValues(endpoint, sender.BasePayloadSender.endpoint)
		assert.EqualValues(cfg, sender.conf)
	})

	t.Run("multi", func(t *testing.T) {
		endpoints := []Endpoint{
			&DatadogEndpoint{Host: "host1", APIKey: "key1"},
			&DatadogEndpoint{Host: "host2", APIKey: "key2"},
			&DatadogEndpoint{Host: "host3", APIKey: "key3"},
		}
		sender, ok := newMultiSender(endpoints, cfg).(*multiSender)
		assert := assert.New(t)
		assert.True(ok)
		assert.Len(sender.senders, 3)
		assert.Equal(3, cap(sender.mch))
		for i := range endpoints {
			s, ok := sender.senders[i].(*QueuablePayloadSender)
			assert.True(ok)
			assert.EqualValues(endpoints[i], s.BasePayloadSender.endpoint)
			assert.EqualValues(cfg, s.conf)
		}
	})
}

func TestMultiSender(t *testing.T) {
	t.Run("Start", func(t *testing.T) {
		mock1 := newMockSender()
		mock2 := newMockSender()
		multi := &multiSender{senders: []PayloadSender{mock1, mock2}, mch: make(chan interface{})}
		multi.Start()
		defer multi.Stop()

		assert := assert.New(t)
		assert.Equal(1, mock1.StartCalls())
		assert.Equal(1, mock2.StartCalls())
	})

	t.Run("Stop", func(t *testing.T) {
		mock1 := newMockSender()
		mock2 := newMockSender()
		multi := &multiSender{senders: []PayloadSender{mock1, mock2}, mch: make(chan interface{})}
		multi.Stop()

		assert := assert.New(t)
		assert.Equal(1, mock1.StopCalls())
		assert.Equal(1, mock2.StopCalls())

		select {
		case <-multi.mch:
		default:
			t.Fatal("monitor channel should be closed")
		}
	})

	t.Run("Send", func(t *testing.T) {
		mock1 := newMockSender()
		mock2 := newMockSender()
		p := &Payload{CreationDate: time.Now(), Bytes: []byte{1, 2, 3}}
		multi := &multiSender{senders: []PayloadSender{mock1, mock2}, mch: make(chan interface{})}
		multi.Send(p)

		assert := assert.New(t)
		assert.Equal(p, mock1.SendCalls()[0])
		assert.Equal(p, mock2.SendCalls()[0])
	})

	t.Run("funnel", func(t *testing.T) {
		mock1 := newMockSender()
		mock2 := newMockSender()
		multi := &multiSender{senders: []PayloadSender{mock1, mock2}, mch: make(chan interface{})}
		multi.Start()
		defer multi.Stop()

		mock1.monitor <- "ping1"
		mock2.monitor <- "ping2"

		msg1 := <-multi.mch
		msg2 := <-multi.mch

		assert.Equal(t, "ping1", msg1.(string))
		assert.Equal(t, "ping2", msg2.(string))
	})
}

func TestMockPayloadSender(t *testing.T) {
	p := &Payload{CreationDate: time.Now(), Bytes: []byte{1, 2, 3}}
	mock := newMockSender()
	mock.Start()
	mock.Start()
	mock.Start()
	mock.Send(p)
	mock.Send(p)
	mock.Stop()

	assert := assert.New(t)
	assert.Equal(3, mock.StartCalls())
	assert.Equal(p, mock.SendCalls()[0])
	assert.Equal(p, mock.SendCalls()[1])
	assert.Equal(1, mock.StopCalls())

	mock.Reset()
	assert.Equal(0, mock.StartCalls())
	assert.Equal(0, mock.StopCalls())
	assert.Len(mock.SendCalls(), 0)
}

var _ PayloadSender = (*mockPayloadSender)(nil)

type mockPayloadSender struct {
	startCalls uint64
	stopCalls  uint64

	mu        sync.Mutex
	sendCalls []*Payload
	monitor   chan interface{}
}

func newMockSender() *mockPayloadSender {
	return &mockPayloadSender{monitor: make(chan interface{})}
}

func (m *mockPayloadSender) Reset() {
	atomic.SwapUint64(&m.startCalls, 0)
	atomic.SwapUint64(&m.stopCalls, 0)
	m.mu.Lock()
	m.sendCalls = m.sendCalls[:0]
	m.monitor = make(chan interface{})
	m.mu.Unlock()
}

func (m *mockPayloadSender) Start() {
	atomic.AddUint64(&m.startCalls, 1)
}

func (m *mockPayloadSender) StartCalls() int {
	return int(atomic.LoadUint64(&m.startCalls))
}

// Stop must be called only once. It closes the monitor channel.
func (m *mockPayloadSender) Stop() {
	atomic.AddUint64(&m.stopCalls, 1)
	close(m.monitor)
}

func (m *mockPayloadSender) StopCalls() int {
	return int(atomic.LoadUint64(&m.stopCalls))
}

func (m *mockPayloadSender) Send(p *Payload) {
	m.mu.Lock()
	m.sendCalls = append(m.sendCalls, p)
	m.mu.Unlock()
}

func (m *mockPayloadSender) SendCalls() []*Payload {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendCalls
}

func (m *mockPayloadSender) Monitor() <-chan interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.monitor
}

func (m *mockPayloadSender) Run()                          {}
func (m *mockPayloadSender) setEndpoint(endpoint Endpoint) {}

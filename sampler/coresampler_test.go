package sampler

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	log "github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

func getTestSampler() *Sampler {
	// Disable debug logs in these tests
	log.UseLogger(log.Disabled)

	// No extra fixed sampling, no maximum TPS
	extraRate := 1.0
	maxTPS := 0.0

	return newSampler(extraRate, maxTPS)
}

func TestSamplerAccessRace(t *testing.T) {
	// regression test: even though the sampler is channel protected, it
	// has getters accessing its fields.
	s := newSampler(1, 2)
	go func() {
		for i := 0; i < 10000; i++ {
			s.SetSignatureCoefficients(float64(i), float64(i)/2)
		}
	}()
	for i := 0; i < 5000; i++ {
		s.GetState()
		s.GetAllCountScores()
	}
}

func TestSamplerLoop(t *testing.T) {
	s := getTestSampler()

	exit := make(chan bool)

	go func() {
		s.Run()
		close(exit)
	}()

	s.Stop()

	select {
	case <-exit:
		return
	case <-time.After(time.Second * 1):
		assert.Fail(t, "Sampler took more than 1 second to close")
	}
}

func TestCombineRates(t *testing.T) {
	var combineRatesTests = []struct {
		rate1, rate2 float64
		expected     float64
	}{
		{0.1, 1.0, 1.0},
		{0.3, 0.2, 0.44},
		{0.0, 0.5, 0.5},
	}
	for _, tt := range combineRatesTests {
		assert.Equal(t, tt.expected, CombineRates(tt.rate1, tt.rate2))
		assert.Equal(t, tt.expected, CombineRates(tt.rate2, tt.rate1))
	}
}

func TestAddSampleRate(t *testing.T) {
	assert := assert.New(t)
	tID := randomTraceID()

	root := model.Span{TraceID: tID, SpanID: 1, ParentID: 0, Start: 123, Duration: 100000, Service: "mcnulty", Type: "web"}

	AddSampleRate(&root, 0.4)
	assert.Equal(0.4, root.Metrics["_sample_rate"], "sample rate should be 40%%")

	AddSampleRate(&root, 0.5)
	assert.Equal(0.2, root.Metrics["_sample_rate"], "sample rate should be 20%% (50%% of 40%%)")
}

// MockEngine mocks a sampler engine
type MockEngine struct {
	wantSampled bool
	wantRate    float64
}

// NewMockEngine returns a MockEngine for tests
func NewMockEngine(wantSampled bool, wantRate float64) *MockEngine {
	return &MockEngine{wantSampled: wantSampled, wantRate: wantRate}
}

// Sample returns a constant rate
func (e *MockEngine) Sample(_ model.Trace, _ *model.Span, _ string) (bool, float64) {
	return e.wantSampled, e.wantRate
}

// Run mocks Engine.Run()
func (e *MockEngine) Run() {
	return
}

// Stop mocks Engine.Stop()
func (e *MockEngine) Stop() {
	return
}

// GetState mocks Engine.GetState()
func (e *MockEngine) GetState() interface{} {
	return nil
}

// GetType mocks Engine.GetType()
func (e *MockEngine) GetType() EngineType {
	return EngineType(0)
}

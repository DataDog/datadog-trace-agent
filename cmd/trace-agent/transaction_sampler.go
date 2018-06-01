package main

import (
	"sync"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/poller"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// TransactionSampler extracts and samples analyzed spans
type TransactionSampler struct {
	analyzed chan *model.Span

	mu                    sync.RWMutex
	analyzedRateByService map[string]float64
}

// NewTransactionSampler creates a new empty transaction sampler
// It must be initalized with startup config, a channel for outbound analyzed spans, and
// a poller.Poller for updating in-memory configuration
func NewTransactionSampler(conf *config.AgentConfig, analyzed chan *model.Span, poller *poller.Poller) *TransactionSampler {
	t := &TransactionSampler{
		analyzed:              analyzed,
		analyzedRateByService: conf.AnalyzedRateByService,
	}

	if poller != nil {
		go t.listen(poller.Updates())
	}

	return t
}

// Enabled tells if the transaction analysis is enabled
func (s *TransactionSampler) Enabled() bool {
	return len(s.analyzedRateByService) > 0
}

// Add extracts analyzed spans and send them to its `analyzed` channel
func (s *TransactionSampler) Add(t processedTrace) {
	// inspect the WeightedTrace so that we can identify top-level spans
	for _, span := range t.WeightedTrace {
		if s.Analyzed(span) {
			s.analyzed <- span.Span
		}
	}
}

func (s *TransactionSampler) ratesByService() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.analyzedRateByService
}

// Analyzed tells if a span should be considered as analyzed
// Only top-level spans are eligible to be analyzed
func (s *TransactionSampler) Analyzed(span *model.WeightedSpan) bool {
	if !span.TopLevel {
		return false
	}

	ratesByService := s.ratesByService()
	if analyzeRate, ok := ratesByService[span.Service]; ok {
		if sampler.SampleByRate(span.TraceID, analyzeRate) {
			return true
		}
	}
	return false
}

// listen listens for new ServerConfig reported by the poller
func (s *TransactionSampler) listen(in <-chan *config.ServerConfig) {
	for {
		select {
		case conf := <-in:
			s.mu.Lock()

			s.analyzedRateByService = conf.AnalyzedRateByService
			s.mu.Unlock()
		default:
		}
	}
}

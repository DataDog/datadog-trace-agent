package main

import (
	"sync"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// TransactionSampler extracts and samples analyzed spans
type TransactionSampler struct {
	analyzed chan *model.Span

	analyzedRateByService map[string]float64
	mu                    sync.RWMutex
}

// NewTransactionSampler creates a new empty transaction sampler
func NewTransactionSampler(conf *config.AgentConfig, analyzed chan *model.Span, config chan *config.ServerConfig) *TransactionSampler {
	t := &TransactionSampler{
		analyzed:              analyzed,
		analyzedRateByService: conf.AnalyzedRateByService,
	}

	go t.listen(config)
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

func (s *TransactionSampler) listen(in chan *config.ServerConfig) {
	for {
		select {
		case conf := <-in:
			func() {
				s.mu.Lock()
				defer s.mu.Unlock()

				// during rollout, don't allow an empty server response to cancel
				// analysis
				if len(conf.AnalyzedRateByService) > 0 {
					s.analyzedRateByService = conf.AnalyzedRateByService
				}

			}()
		default:
		}
	}
}

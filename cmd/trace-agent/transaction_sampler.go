package main

import (
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// TransactionSampler extracts and samples analyzed spans
type TransactionSampler struct {
	analyzed              chan *model.Span
	analyzedRateByService map[string]float64
}

// NewTransactionSampler creates a new empty transaction sampler
func NewTransactionSampler(conf *config.AgentConfig, analyzed chan *model.Span, config chan *config.ServerConfig) *TransactionSampler {
	t := &TransactionSampler{
		analyzed:              analyzed,
		analyzedRateByService: conf.AnalyzedRateByService,
	}

	go t.Listen(config)
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

// Analyzed tells if a span should be considered as analyzed
// Only top-level spans are eligible to be analyzed
func (s *TransactionSampler) Analyzed(span *model.WeightedSpan) bool {
	if !span.TopLevel {
		return false
	}

	if analyzeRate, ok := s.analyzedRateByService[span.Service]; ok {
		if sampler.SampleByRate(span.TraceID, analyzeRate) {
			return true
		}
	}
	return false
}

func (s *TransactionSampler) Listen(in chan *config.ServerConfig) error {
	// TODO[aaditya] when to actually return an error
	for {
		select {
		case conf := <-in:
			//TODO[aaditya]: lock here?
			s.analyzedRateByService = conf.AnalyzedServices
		default:
		}
	}

	return nil
}

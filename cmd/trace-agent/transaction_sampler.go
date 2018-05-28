package main

import (
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// TransactionSampler extracts and samples analyzed spans
type TransactionSampler interface {
	// Enabled tells if the transaction analysis is enabled
	Enabled() bool
	// Add extracts analyzed spans and send them to its `analyzed` channel
	Add(processedTrace)
}

// NewTransactionSampler creates a new empty transaction sampler
func NewTransactionSampler(conf *config.AgentConfig, analyzed chan *model.Span) TransactionSampler {
	if len(conf.AnalyzedSpansByService) > 0 {
		return newTransactionSampler(conf.AnalyzedSpansByService, analyzed)
	}
	if len(conf.AnalyzedRateByServiceLegacy) > 0 {
		return newLegacyTransactionSampler(conf.AnalyzedRateByServiceLegacy, analyzed)
	}
	return &disabledTransactionSampler{}
}

type disabledTransactionSampler struct {
}

func (s *disabledTransactionSampler) Enabled() bool {
	return false
}

func (s *disabledTransactionSampler) Add(t processedTrace) {
}

type transactionSampler struct {
	analyzed               chan *model.Span
	analyzedSpansByService map[string]map[string]float64
}

func newTransactionSampler(analyzedSpansByService map[string]map[string]float64, analyzed chan *model.Span) *transactionSampler {
	return &transactionSampler{
		analyzed:               analyzed,
		analyzedSpansByService: analyzedSpansByService,
	}
}

// Enabled tells if the transaction analysis is enabled
func (s *transactionSampler) Enabled() bool {
	return true
}

// Add extracts analyzed spans and send them to its `analyzed` channel
func (s *transactionSampler) Add(t processedTrace) {
	// inspect the WeightedTrace so that we can identify top-level spans
	for _, span := range t.WeightedTrace {
		if s.shouldAnalyze(span) {
			s.analyzed <- span.Span
		}
	}
}

func (s *transactionSampler) shouldAnalyze(span *model.WeightedSpan) bool {
	if operations, ok := s.analyzedSpansByService[span.Service]; ok {
		if analyzeRate, ok := operations[span.Name]; ok {
			if sampler.SampleByRate(span.TraceID, analyzeRate) {
				return true
			}
		}
	}
	return false
}

type legacyTransactionSampler struct {
	analyzed              chan *model.Span
	analyzedRateByService map[string]float64
}

func newLegacyTransactionSampler(analyzedRateByService map[string]float64, analyzed chan *model.Span) *legacyTransactionSampler {
	return &legacyTransactionSampler{
		analyzed:              analyzed,
		analyzedRateByService: analyzedRateByService,
	}
}

// Enabled tells if the transaction analysis is enabled
func (s *legacyTransactionSampler) Enabled() bool {
	return true
}

// Add extracts analyzed spans and send them to its `analyzed` channel
func (s *legacyTransactionSampler) Add(t processedTrace) {
	// inspect the WeightedTrace so that we can identify top-level spans
	for _, span := range t.WeightedTrace {
		if s.shouldAnalyze(span) {
			s.analyzed <- span.Span
		}
	}
}

// shouldAnalyze tells if a span should be considered as analyzed
// Only top-level spans are eligible to be analyzed
func (s *legacyTransactionSampler) shouldAnalyze(span *model.WeightedSpan) bool {
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

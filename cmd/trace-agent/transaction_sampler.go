package main

import (
	"github.com/DataDog/datadog-trace-agent/agent"
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// TransactionSampler filters and samples interesting spans in a trace based on implementation specific criteria.
type TransactionSampler interface {
	// Extract extracts matching spans from the given trace and returns them.
	Extract(t processedTrace) []*agent.Span
}

// NewTransactionSampler creates a new empty transaction sampler
func NewTransactionSampler(conf *config.AgentConfig) TransactionSampler {
	if len(conf.AnalyzedSpansByService) > 0 {
		return newTransactionSampler(conf.AnalyzedSpansByService)
	}
	if len(conf.AnalyzedRateByServiceLegacy) > 0 {
		return newLegacyTransactionSampler(conf.AnalyzedRateByServiceLegacy)
	}
	return &disabledTransactionSampler{}
}

type disabledTransactionSampler struct{}

func (s *disabledTransactionSampler) Extract(t processedTrace) []*agent.Span {
	return nil
}

type transactionSampler struct {
	analyzedSpansByService map[string]map[string]float64
}

func newTransactionSampler(analyzedSpansByService map[string]map[string]float64) *transactionSampler {
	return &transactionSampler{
		analyzedSpansByService: analyzedSpansByService,
	}
}

// Extract extracts analyzed spans and returns them as a slice
func (s *transactionSampler) Extract(t processedTrace) []*agent.Span {
	var transactions []*agent.Span

	// Get the trace priority
	priority, hasPriority := t.getSamplingPriority()
	// inspect the WeightedTrace so that we can identify top-level spans
	for _, span := range t.WeightedTrace {
		if s.shouldAnalyze(span, hasPriority, priority) {
			transactions = append(transactions, span.Span)
		}
	}

	return transactions
}

func (s *transactionSampler) shouldAnalyze(span *agent.WeightedSpan, hasPriority bool, priority int) bool {
	if operations, ok := s.analyzedSpansByService[span.Service]; ok {
		if analyzeRate, ok := operations[span.Name]; ok {
			// If the trace has been manually sampled, we keep all matching spans
			highPriority := hasPriority && priority >= 2
			if highPriority || sampler.SampleByRate(span.TraceID, analyzeRate) {
				return true
			}
		}
	}
	return false
}

type legacyTransactionSampler struct {
	analyzedRateByService map[string]float64
}

func newLegacyTransactionSampler(analyzedRateByService map[string]float64) *legacyTransactionSampler {
	return &legacyTransactionSampler{
		analyzedRateByService: analyzedRateByService,
	}
}

// Extract extracts analyzed spans and returns them as a slice
func (s *legacyTransactionSampler) Extract(t processedTrace) []*agent.Span {
	var transactions []*agent.Span

	// inspect the WeightedTrace so that we can identify top-level spans
	for _, span := range t.WeightedTrace {
		if s.shouldAnalyze(span) {
			transactions = append(transactions, span.Span)
		}
	}

	return transactions
}

// shouldAnalyze tells if a span should be considered as analyzed
// Only top-level spans are eligible to be analyzed
func (s *legacyTransactionSampler) shouldAnalyze(span *agent.WeightedSpan) bool {
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

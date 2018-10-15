package main

import (
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// TransactionSampler filters and samples interesting spans in a trace based on implementation specific criteria.
type TransactionSampler interface {
	// Extract extracts matching spans from the given trace and returns them.
	Extract(t processedTrace) []*model.Span
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

func (s *disabledTransactionSampler) Extract(t processedTrace) []*model.Span {
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
func (s *transactionSampler) Extract(t processedTrace) []*model.Span {
	var transactions []*model.Span

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

func (s *transactionSampler) shouldAnalyze(span *model.WeightedSpan, hasPriority bool, priority int) bool {
	var analyzeRate float64

	// Read sample rate from span metrics.
	if rate, ok := span.Span.Metrics[sampler.EventSampleRateKey]; ok {
		analyzeRate = rate
	} else {
		// If not available, fallback to Agent-configured rates.
		if operations, ok := s.analyzedSpansByService[span.Service]; ok {
			if rate, ok := operations[span.Name]; ok {
				analyzeRate = rate
				// Update the stored sample rate to unify instrumentation-provided and agent-provided configuration.
				span.Span.SetMetric(sampler.EventSampleRateKey, analyzeRate)
			}
		}
	}

	if analyzeRate > 0 {
		// If the trace has been manually sampled, we keep all matching spans. We also update the sample rate stored
		// to reflect it.
		highPriority := hasPriority && priority >= 2
		if highPriority {
			span.Span.SetMetric(sampler.EventSampleRateKey, 1)
			return true
		}

		// The common case is to sample based on the rate.
		if sampler.SampleByRate(span.TraceID, analyzeRate) {
			return true
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
func (s *legacyTransactionSampler) Extract(t processedTrace) []*model.Span {
	var transactions []*model.Span

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

package filters

import (
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"math/rand"
)

// Transaction is a rule for capturing transactions
type Transaction struct {
	transactionType string

	service   string
	operation string

	sampleRate float64
}

// ALL analyzes everything
const ALL = "*"

// WebOnly analyzes only web endpoints
var WebOnly = Transaction{"http", ALL, ALL, 1}

// McnultyOnly ensures support-admin requests are analyzed
var McnultyOnly = Transaction{ALL, "support-admin", "pylons.request", 1}

// Matches matches a transaction against rules
func (t *Transaction) Matches(s *model.Span) bool {
	var typeMatches, serviceMatches, operationMatches, sampled bool

	typeMatches = t.transactionType == ALL
	if !typeMatches && t.transactionType != "" {
		typeMatches = t.transactionType == s.Type
	}
	serviceMatches = t.service == ALL
	if !serviceMatches && t.service != "" {
		serviceMatches = t.service == s.Service
	}

	operationMatches = t.operation == ALL
	if !operationMatches && t.operation != "" {
		operationMatches = t.operation == s.Name
	}

	sampled = float64(rand.Intn(100)) < (100.0 * t.sampleRate)
	return typeMatches && serviceMatches && operationMatches && sampled
}

// TransactionFilter implements a filter based on span levels
type TransactionFilter struct {
	analyzed []Transaction
	rejected []Transaction
}

// Keep returns true if SpanLevel is at or above the cutoff level
func (f *TransactionFilter) Keep(s *model.Span) bool {
	return !f.Rejected(s) && f.Analyzed(s)
}

// Rejected matches on rejected rules
func (f *TransactionFilter) Rejected(s *model.Span) bool {
	for _, fil := range f.rejected {
		if fil.Matches(s) {
			return true
		}
	}

	return false
}

// Analyzed matches on analyzed rules
func (f *TransactionFilter) Analyzed(s *model.Span) bool {
	for _, fil := range f.analyzed {
		if fil.Matches(s) {
			return true
		}
	}

	return false
}

// NewTransactionFilter creates a new transaction filter
func NewTransactionFilter(conf *config.AgentConfig) Filter {
	analyzed := []Transaction{}
	if conf.AnalyzeWebTransactions {
		analyzed = append(analyzed, WebOnly)
		analyzed = append(analyzed, McnultyOnly)
	}

	// TODO: support rejected
	return &TransactionFilter{
		analyzed: analyzed,
		rejected: []Transaction{},
	}
}

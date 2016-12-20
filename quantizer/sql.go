package quantizer

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	sqlQueryTag      = "sql.query"
	sqlQuantizeError = "agent.parse.error"
)

// sets the filters used in the QuantizeSQL process
var sqlFilters = []TokenFilter{
	&DiscardFilter{},
	&ReplaceFilter{},
	&GroupingFilter{},
}

// is the token consumer that will quantize the query
var tokenQuantizer = NewTokenConsumer(sqlFilters)

// QuantizeSQL generates resource and sql.query meta for SQL spans
func QuantizeSQL(span model.Span) model.Span {
	if span.Resource == "" {
		return span
	}

	log.Debugf("Quantize SQL command, generate resource from the query, SpanID: %d", span.SpanID)
	quantizedString, err := tokenQuantizer.Process(span.Resource)

	if err != nil {
		// if we have an error, the partially parsed SQL is discarded so that we don't pollute
		// users resources. Here we provide more details to debug the problem.
		log.Debugf("Error parsing the query: `%s`", span.Resource)
		span.Resource = "Non-parsable SQL query"

		if span.Meta == nil {
			span.Meta = make(map[string]string)
		}

		span.Meta[sqlQuantizeError] = "Query not parsed"
		return span
	}

	span.Resource = quantizedString

	// set the sql.query tag if and only if it's not already set by users. If a users set
	// this value, we send that value AS IS to the backend. If the value is not set, we
	// try to obfuscate users parameters so that sensitive data are not sent in the backend.
	// TODO: the current implementation is a rough approximation that assumes
	// obfuscation == quantization. This is not true in real environments because we're
	// removing data that could be interesting for users.
	if span.Meta != nil && span.Meta[sqlQueryTag] != "" {
		return span
	}

	if span.Meta == nil {
		span.Meta = make(map[string]string)
	}

	span.Meta[sqlQueryTag] = quantizedString
	return span
}

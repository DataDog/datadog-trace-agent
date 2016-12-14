package quantizer

import (
	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

const (
	sqlVariableReplacement = "?"
	sqlQueryTag            = "sql.query"
)

// sets the filters used in the QuantizeSQL process
var sqlFilters = []TokenFilter{
	&DiscardFilter{},
	&ReplaceFilter{},
}

// is the token consumer that will quantize the query
var tokenQuantizer = NewTokenConsumer(sqlFilters)

// QuantizeSQL generates resource and sql.query meta for SQL spans
func QuantizeSQL(span model.Span) model.Span {
	if span.Resource == "" {
		return span
	}

	// start quantization
	log.Debugf("Quantize SQL command, generate resource from the query, SpanID: %d", span.SpanID)
	resource, err := tokenQuantizer.Process(span.Resource)
	if err != nil {
		// TODO[manu]: the quantization has found a LEX_ERROR and we should decide
		// the best strategy to address the limit of our quantizer
		return span
	}

	span.Resource = resource

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

	span.Meta[sqlQueryTag] = resource
	return span
}

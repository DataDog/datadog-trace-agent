package quantizer

import (
	"regexp"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

//
var cqlListVariablesRegex = regexp.MustCompile(`(%s[,]?)+`) // (%s, %s, %s, %s)

// QuantizeCassandra generates resources for Cassandra spans
func QuantizeCassandra(span model.Span) model.Span {
	_, ok := span.Meta["query"]
	if !ok {
		log.Debugf("`query` meta is missing in a Cassandra span, can't quantize it, SpanID: %d", span.SpanID)
		return span
	}
	// First treat the span as SQL and normalize it
	span = QuantizeSQL(span)
	resource := span.Resource

	// Replace parenthesized variable lists (%s, %s, %s, %s)
	resource = cqlListVariablesRegex.ReplaceAllString(resource, sqlVariableReplacement)

	span.Resource = resource
	return span
}

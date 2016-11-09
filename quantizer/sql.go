package quantizer

import (
	"regexp"
	"strings"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

const (
	sqlVariableReplacement = "?"
	sqlQueryTag            = "sql.query"
)

var sqlVariablesRegexp = regexp.MustCompile("('[^']+')|([\\$]*\\b[0-9]+\\b)")
var sqlLiteralsRegexp = regexp.MustCompile("\\b(?i:true|false|null)\\b")
var sqlalchemyVariablesRegexp = regexp.MustCompile("%\\(.+?\\)s")
var sqlListVariablesRegexp = regexp.MustCompile("\\?[\\? ,]+\\?")
var sqlCommentsRegexp = regexp.MustCompile("--[^\n]*")

// CQL encodes query params with %s
var cqlListVariablesRegex = regexp.MustCompile(`%s(([,\s]|%s)+%s\s*|)`) // (%s, %s, %s, %s)

// QuantizeSQL generates resource and sql.query meta for SQL spans
func QuantizeSQL(span model.Span) model.Span {
	// Trim spaces and ending special chars
	query := span.Resource

	// remove special characters to get a clean query
	query = strings.TrimSpace(query)
	query = strings.TrimSuffix(query, ";")

	if query == "" {
		span.Resource = query
		return span
	}

	log.Debugf("Quantize SQL command, generate resource from the query, SpanID: %d", span.SpanID)
	resource := quantize(query)
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

// quantize returns a quantized string for the given query
func quantize(query string) string {
	// Remove variables
	query = sqlVariablesRegexp.ReplaceAllString(query, sqlVariableReplacement)
	query = sqlLiteralsRegexp.ReplaceAllString(query, sqlVariableReplacement)
	query = sqlalchemyVariablesRegexp.ReplaceAllString(query, sqlVariableReplacement)

	// Deal with list of variables of arbitrary size
	query = sqlListVariablesRegexp.ReplaceAllString(query, sqlVariableReplacement)

	// Remove comments
	query = sqlCommentsRegexp.ReplaceAllString(query, "")

	// Uniform spacing
	query = compactAllSpaces(query)

	// Replace parenthesized variable lists (%s, %s, %s, %s)
	query = cqlListVariablesRegex.ReplaceAllString(query, sqlVariableReplacement)

	return query
}

package quantizer

import (
	"regexp"
	"strings"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

const sqlVariableReplacement = "?"

var sqlVariablesRegexp = regexp.MustCompile("('[^']+')|([\\$]*[0-9]+)")
var sqlalchemyVariablesRegexp = regexp.MustCompile("%\\(.+?\\)s")
var sqlListVariablesRegexp = regexp.MustCompile("\\?[\\? ,]+\\?")
var sqlCommentsRegexp = regexp.MustCompile("--[^\n]*")

// CQL encodes query params with %s
var cqlListVariablesRegex = regexp.MustCompile(`%s(([,\s]|%s)+%s\s*|)`) // (%s, %s, %s, %s)

// QuantizeSQL generates resource for SQL spans
func QuantizeSQL(span model.Span) model.Span {
	// Trim spaces and ending special chars
	query := span.Resource

	resource := strings.TrimSpace(query)
	resource = strings.TrimSuffix(resource, ";")

	if resource == "" {
		span.Resource = resource
		return span
	}

	log.Debugf("Quantize SQL command, generate resource from the query, SpanID: %d", span.SpanID)

	// Remove variables
	resource = sqlVariablesRegexp.ReplaceAllString(resource, sqlVariableReplacement)
	resource = sqlalchemyVariablesRegexp.ReplaceAllString(resource, sqlVariableReplacement)

	// Deal with list of variables of arbitrary size
	resource = sqlListVariablesRegexp.ReplaceAllString(resource, sqlVariableReplacement)

	// Remove comments
	resource = sqlCommentsRegexp.ReplaceAllString(resource, "")

	// Uniform spacing
	resource = compactAllSpaces(resource)

	// Replace parenthesized variable lists (%s, %s, %s, %s)
	resource = cqlListVariablesRegex.ReplaceAllString(resource, sqlVariableReplacement)

	span.Resource = resource

	return span
}

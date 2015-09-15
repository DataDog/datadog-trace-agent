package quantizer

import (
	"regexp"
	"strings"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

const sqlVariableReplacement = "?"

var sqlVariablesRegexp = regexp.MustCompile("('[^']+')|([0-9]+)")
var sqlalchemyVariablesRegexp = regexp.MustCompile("%\\(.+?\\)s")
var sqlListVariablesRegexp = regexp.MustCompile("\\?[\\? ,]+\\?")
var sqlCommentsRegexp = regexp.MustCompile("--[^\n]*")

// QuantizeSQL generates resource for SQL spans
func QuantizeSQL(span model.Span) model.Span {
	query, ok := span.Meta["query"]
	if !ok {
		log.Debugf("`query` meta is missing in a SQL span, can't quantize it, SpanID: %d", span.SpanID)
		return span
	}

	resource := strings.TrimSpace(query)

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

	span.Resource = resource

	return span
}

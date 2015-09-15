package quantizer

import (
	"regexp"
	"strings"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

const sqlVariableReplacement = "?"

var sqlCommentName = regexp.MustCompile("^-- ([^\n]*)")
var sqlVariablesRegexp = regexp.MustCompile("('[^']+')|([0-9]+)")
var sqlalchemyVariablesRegexp = regexp.MustCompile("%\\(.+?\\)s")
var sqlListRegexp = regexp.MustCompile("('[^']+')|([0-9]+)")
var sqlListVariables = regexp.MustCompile("\\?[\\? ,]+\\?")

// QuantizeSQL generates resource for SQL spans
func QuantizeSQL(span model.Span) model.Span {
	query, ok := span.Meta["query"]
	if !ok {
		log.Debugf("`query` meta is missing in a SQL span, can't quantize it, SpanID: %d", span.SpanID)
		return span
	}

	resource := strings.TrimSpace(query)

	log.Debugf("Quantize SQL command, generate resource from the query, SpanID: %d", span.SpanID)

	resource = compactAllSpaces(resource)

	// Remove variables
	resource = sqlVariablesRegexp.ReplaceAllString(resource, sqlVariableReplacement)
	resource = sqlalchemyVariablesRegexp.ReplaceAllString(resource, sqlVariableReplacement)

	// Deal with list of variables of arbitrary size
	resource = sqlListVariables.ReplaceAllString(resource, sqlVariableReplacement)

	span.Resource = resource

	return span
}

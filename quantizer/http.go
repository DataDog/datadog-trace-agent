package quantizer

import (
	"net/url"

	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	httpURLMeta       = "http.url"
	httpQuantizeError = "agent.parse.error"
)

// QuantizeHTTP extracts a URL stem for HTTP spans
func QuantizeHTTP(span model.Span) model.Span {
	// the URL should be located in the resource
	rawurl := span.Resource

	if span.Meta == nil {
		span.Meta = make(map[string]string)
	}
	span.Meta[httpURLMeta] = rawurl

	u, err := url.Parse(rawurl)
	if err != nil || (u.Scheme == "" && u.Host == "") {
		span.Resource = "Non-parsable URL"
		span.Meta[httpQuantizeError] = "Problem when parsing URL"
		return span
	}

	if span.Meta == nil {
		span.Meta = make(map[string]string)
	}

	span.Resource = u.Scheme + "://" + u.Host
	return span
}

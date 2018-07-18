package obfuscate

import (
	"net/url"
	"strings"

	"github.com/DataDog/datadog-trace-agent/model"
)

// httpReplaceQueryRegexp is the regular expression used to replace
// query strings after HTTP URLs.
const httpReplaceQueryRegexp = `\?.*$`

func (o *Obfuscator) obfuscateHTTP(span *model.Span) {
	if span.Meta == nil {
		return
	}
	if !o.opts.HTTP.RemoveQueryString && !o.opts.HTTP.RemovePathDigits {
		// nothing to do
		return
	}
	const k = "http.url"
	val, ok := span.Meta[k]
	if !ok {
		return
	}
	uri, err := url.Parse(val)
	if err != nil {
		// should not happen for valid URLs, but better obfuscate everything
		// rather than exposing sensitive information when this option
		// is set.
		span.Meta[k] = "?"
		return
	}
	if o.opts.HTTP.RemoveQueryString {
		uri.ForceQuery = true // add the '?'
		uri.RawQuery = ""
	}
	if o.opts.HTTP.RemovePathDigits {
		segs := strings.Split(uri.Path, "/")
		var changed bool
		for i, seg := range segs {
			if strings.IndexAny(seg, "0123456789") > -1 {
				segs[i] = "?"
				changed = true
			}
		}
		if changed {
			uri.Path = strings.Join(segs, "/")
		}
	}
	span.Meta[k] = uri.String()
}

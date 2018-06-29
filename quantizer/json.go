package quantizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
)

// obfuscateES obfuscates ElasticSearch JSON body values.
func (o *Obfuscator) obfuscateES(span *model.Span) {
	if !o.opts.ES.Enabled {
		return
	}
	quantizeJSON(&o.opts.ES, span, "elasticsearch.body")
}

// obfuscateMongo obfuscates MongoDB JSON query values.
func (o *Obfuscator) obfuscateMongo(span *model.Span) {
	if !o.opts.Mongo.Enabled {
		return
	}
	quantizeJSON(&o.opts.Mongo, span, "mongodb.query")
}

// quantizeJSON obfuscates JSON key values in the span's meta tag using the configuration from cfg.
func quantizeJSON(cfg *config.JSONObfuscationConfig, span *model.Span, tag string) {
	if span.Meta == nil || span.Meta[tag] == "" {
		return
	}
	span.Meta[tag], _ = newJSONObfuscator(cfg).obfuscate(span.Meta[tag])
	// we should accept whatever the obfuscator returns, even if it's an error: a parsing
	// error simply means that the JSON was invalid, meaning that we've only obfuscated
	// as much of it as we could. This happens in cases when the JSON body (for example in the
	// case of "elasticsearch.body" tag) is truncated, rendering the JSON invalid.
}

type jsonObfuscator struct {
	keepValue map[string]bool // do not obfuscate values for these keys
	isKey     bool            // next token is a key
	keeping   bool            // true if not obfuscating
	keepDepth int             // depth after which we've stopped obfuscating
	closures  []bool          // parent closure count, false if array (e.g. {[ => []bool{true, false})
	out       bytes.Buffer    // resulting JSON
	dec       *json.Decoder   // decoder
}

func newJSONObfuscator(cfg *config.JSONObfuscationConfig) *jsonObfuscator {
	if cfg.KeepMap == nil {
		keepValue := make(map[string]bool, len(cfg.KeepValues))
		for _, v := range cfg.KeepValues {
			keepValue[v] = true
		}
		cfg.KeepMap = keepValue
	}
	return &jsonObfuscator{
		keepValue: cfg.KeepMap,
		closures:  []bool{},
	}
}

func (tok *jsonObfuscator) scanDelim(v json.Delim) {
	switch v {
	case '[':
		tok.closures = append(tok.closures, false)
		tok.isKey = false
		tok.out.WriteString(string(v))
	case '{':
		tok.closures = append(tok.closures, true)
		tok.isKey = true
		tok.out.WriteString(string(v))
	case ']', '}':
		tok.closures = tok.closures[:len(tok.closures)-1]
		tok.isKey = tok.isObject()
		tok.out.WriteString(string(v))
		if tok.dec.More() {
			tok.out.WriteString(",")
		}
	}
}

func (tok *jsonObfuscator) scanKey(t json.Token) string {
	var k string
	switch v := t.(type) {
	case string:
		k = v
	default:
		k = fmt.Sprint(v)
	}
	tok.out.WriteString(`"`)
	tok.out.WriteString(k)
	tok.out.WriteString(`":`)
	return k
}

func (tok *jsonObfuscator) scanValue(t json.Token) {
	if !tok.keeping {
		tok.out.WriteString(`"?"`)
	} else {
		switch v := t.(type) {
		case bool:
			if v {
				tok.out.WriteString(`"true"`)
			} else {
				tok.out.WriteString(`"false"`)
			}
		case float64:
			tok.out.WriteString(strconv.FormatFloat(v, 'f', 2, 64))
		case json.Number:
			tok.out.WriteString(string(v))
		case string:
			tok.out.WriteString(`"`)
			tok.out.WriteString(v)
			tok.out.WriteString(`"`)
		case nil:
			tok.out.WriteString(`"null"`)
		}
	}
	if tok.dec.More() {
		tok.out.WriteString(",")
	}
}

// isObject reports whether the current closure is an object.
func (tok *jsonObfuscator) isObject() bool {
	return len(tok.closures) == 0 || tok.closures[len(tok.closures)-1]
}

// obfuscateJSON takes the JSON string found in str, replacing all the values of the keys found
// as keys in the drop map with a "?" and returning the new JSON string.
func (tok *jsonObfuscator) obfuscate(str string) (string, error) {
	tok.dec = json.NewDecoder(strings.NewReader(str))
	tok.dec.UseNumber()

	for {
		t, err := tok.dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// it might be that a truncated JSON was passed, in which
			// case the tokenizer failed to process it entirely, we
			// should return whatever we have.
			tok.out.WriteString("...")
			return tok.out.String(), err
		}
		if v, ok := t.(json.Delim); ok {
			tok.scanDelim(v)
			continue
		}
		depth := len(tok.closures)
		if tok.isKey {
			k := tok.scanKey(t)
			if !tok.keeping && tok.keepValue[k] {
				// start keeping values
				tok.keeping = true
				tok.keepDepth = depth + 1
				tok.isKey = false
				continue
			}
		} else {
			tok.scanValue(t)
		}
		if tok.keeping && depth < tok.keepDepth {
			// we've come back to the source closure, stop keeping
			tok.keeping = false
		}
		if tok.isObject() {
			tok.isKey = !tok.isKey
		}
	}

	return tok.out.String(), nil
}

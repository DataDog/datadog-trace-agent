package quantizer

import (
	"bytes"
	"encoding/json"
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
	str, err := newJSONObfuscator(cfg).obfuscate(span.Meta[tag])
	if err != nil {
		return
	}
	span.Meta[tag] = str
}

type jsonObfuscator struct {
	keepValue map[string]bool // do not obfuscate values for these keys
	isKey     bool            // next token is a key
	prevKey   string          // last/current key
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

func (tok *jsonObfuscator) scanToken(t json.Token) {
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
		if tok.isKey {
			tok.prevKey = v
		}
		tok.out.WriteString(`"`)
		tok.out.WriteString(v)
		tok.out.WriteString(`"`)
	case nil:
		tok.out.WriteString(`"null"`)
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
			return "", err
		}
		if v, ok := t.(json.Delim); ok {
			// delimiter
			tok.scanDelim(v)
			continue
		}
		if tok.isKey {
			// key
			tok.scanToken(t)
			tok.out.WriteString(":")
		} else {
			// value
			if !tok.keepValue[tok.prevKey] {
				tok.out.WriteString(`"?"`)
			} else {
				tok.scanToken(t)
			}
			if tok.dec.More() {
				tok.out.WriteString(",")
			}
		}
		if tok.isObject() {
			tok.isKey = !tok.isKey
		}
	}

	return tok.out.String(), nil
}

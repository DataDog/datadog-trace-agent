package quantizer

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
)

// QuantizeES obfuscates ElasticSearch JSON body values.
func QuantizeES(cfg *config.AgentConfig, span *model.Span) {
	if cfg.Obfuscation == nil || !cfg.Obfuscation.ES.Enabled {
		return
	}
	quantizeJSON(&cfg.Obfuscation.ES, span, "elasticsearch.body")
}

// QuantizeMongo obfuscates MongoDB JSON query values.
func QuantizeMongo(cfg *config.AgentConfig, span *model.Span) {
	if cfg.Obfuscation == nil || !cfg.Obfuscation.Mongo.Enabled {
		return
	}
	quantizeJSON(&cfg.Obfuscation.Mongo, span, "mongodb.query")
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
	keepValue  map[string]bool // keep the values for these keys
	isKey      bool            // true if next token is a key
	isObject   bool            // true if closure is an object
	lastKey    string          // previous key
	lastObject int             // depth of nearest object
	depth      int             // current depth
}

func newJSONObfuscator(cfg *config.JSONObfuscationConfig) *jsonObfuscator {
	keepValue := make(map[string]bool, len(cfg.KeepValues))
	// TODO: parse this much earlier, not on every call
	for _, v := range cfg.KeepValues {
		keepValue[v] = true
	}
	return &jsonObfuscator{keepValue: keepValue}
}

func (tok *jsonObfuscator) beginArray() {
	tok.depth++
	tok.isKey = false
	tok.isObject = false
}

func (tok *jsonObfuscator) beginObject() {
	tok.depth++
	tok.isKey = true
	tok.isObject = true
	tok.lastObject = tok.depth
}

func (tok *jsonObfuscator) endClosure() {
	tok.depth--
	tok.isKey = true
	if tok.lastObject == tok.depth {
		// TODO: this is not reliable when nesting many things,
		// we should use a stack?
		// the wrapping parent is not an array
		tok.isObject = true
	}
}

// obfuscateJSON takes the JSON string found in str, replacing all the values of the keys found
// as keys in the drop map with a "?" and returning the new JSON string.
func (tok *jsonObfuscator) obfuscate(str string) (string, error) {
	var res strings.Builder
	dec := json.NewDecoder(strings.NewReader(str))
	dec.UseNumber()
	//log.Printf("%15s %6s %6s %3s %s\n", "Token", "Key", "Object", "Depth", "Last Key")
	for {
		t, err := dec.Token()
		//log.Printf("%15q %6v %6v %3d %q\n", t, key, object, depth, lastKey)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if v, ok := t.(json.Delim); ok {
			switch v {
			case '[':
				tok.beginArray()
				res.WriteString(string(v))
			case '{':
				tok.beginObject()
				res.WriteString(string(v))
			case ']', '}':
				tok.endClosure()
				res.WriteString(string(v))
				if dec.More() {
					res.WriteString(",")
				}
			}
			continue
		}
		if !tok.isKey && !tok.keepValue[tok.lastKey] {
			res.WriteString(`"?"`)
		} else {
			switch v := t.(type) {
			case bool:
				if v {
					res.WriteString(`"true"`)
				} else {
					res.WriteString(`"false"`)
				}
			case float64:
				res.WriteString(strconv.FormatFloat(v, 'f', 2, 64))
			case json.Number:
				res.WriteString(string(v))
			case string:
				if tok.isKey {
					tok.lastKey = v
				}
				res.WriteString(`"`)
				res.WriteString(v)
				res.WriteString(`"`)
			case nil:
				res.WriteString(`"null"`)
			}
		}
		if tok.isKey {
			res.WriteString(":")
		} else {
			if dec.More() {
				res.WriteString(",")
			}
		}
		if tok.isObject {
			tok.isKey = !tok.isKey
		}
	}
	return res.String(), nil
}

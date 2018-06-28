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
	keepValues := make(map[string]bool, len(cfg.KeepValues))
	for _, v := range cfg.KeepValues {
		keepValues[v] = true
	}
	str, err := obfuscateJSON(span.Meta[tag], keepValues)
	if err != nil {
		return
	}
	span.Meta[tag] = str
}

// obfuscateJSON takes the JSON string found in str, replacing all the values of the keys found
// as keys in the drop map with a "?" and returning the new JSON string.
func obfuscateJSON(str string, drop map[string]bool) (string, error) {
	var (
		key       bool         // true if token is a key
		object    bool         // true if token is inside an object (as opposed to an array where we don't have keys)
		depth     int          // tokenizer depth
		dropping  bool         // true if we are dropping
		dropDepth int          // the depth at which we are dropping
		res       bytes.Buffer // result
	)
	dec := json.NewDecoder(strings.NewReader(str))
	dec.UseNumber()
	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if v, ok := t.(json.Delim); ok {
			if v == '[' || v == '{' {
				// array or object starting
				key = v == '{'
				object = v != '['
				depth++
				if !dropping {
					res.WriteString(string(v))
				}
			} else {
				// array or object ending
				key = true
				depth--
				if !dropping {
					res.WriteString(string(v))
					if dec.More() {
						res.WriteString(",")
					}
				}
			}
			if !dropping {
				continue
			}
		}
		if dropping {
			if depth < dropDepth {
				res.WriteString(`"?"`)
				if dec.More() {
					res.WriteString(",")
				}
				dropping = false
				key = true
			}
			continue
		}
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
			if key {
				if _, ok := drop[v]; ok {
					dropping = true
					dropDepth = depth + 1
				}
			}
			res.WriteString(`"`)
			res.WriteString(v)
			res.WriteString(`"`)
		case nil:
			res.WriteString(`"null"`)
		}
		if key {
			res.WriteString(":")
		} else {
			if dec.More() {
				res.WriteString(",")
			}
		}
		if object {
			key = !key
		}
	}
	return res.String(), nil
}

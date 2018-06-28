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
	str, err := obfuscateJSON(span.Meta[tag], cfg)
	if err != nil {
		return
	}
	span.Meta[tag] = str
}

// obfuscateJSON takes the JSON string found in str, replacing all the values of the keys found
// as keys in the drop map with a "?" and returning the new JSON string.
func obfuscateJSON(str string, cfg *config.JSONObfuscationConfig) (string, error) {
	keepValue := make(map[string]bool, len(cfg.KeepValues))
	// TODO: do this only once
	for _, v := range cfg.KeepValues {
		keepValue[v] = true
	}
	var (
		key        bool            // true if token is a key
		lastKey    string          // previous key
		object     bool            // true if token is inside an object (as opposed to an array where we don't have keys)
		lastObject int             // depth of nearest object
		depth      int             // current depth
		res        strings.Builder // result
	)
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
			if v == '[' || v == '{' {
				// array or object starting
				depth++
				key = v == '{'
				object = v != '['
				if object {
					lastObject = depth
				}
				res.WriteString(string(v))
			} else {
				// array or object ending
				depth--
				key = true
				if lastObject == depth {
					// the wrapping parent is not an array
					object = true
				}
				res.WriteString(string(v))
				if dec.More() {
					res.WriteString(",")
				}
			}
			continue
		}
		if !key && !keepValue[lastKey] {
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
				if key {
					lastKey = v
				}
				res.WriteString(`"`)
				res.WriteString(v)
				res.WriteString(`"`)
			case nil:
				res.WriteString(`"null"`)
			}
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

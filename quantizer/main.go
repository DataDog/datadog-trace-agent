// Package quantizer implements quantizing and obfuscating of tags and resources for
// a set of spans matching a certain criteria.
package quantizer

import (
	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
)

// Obfuscator quantizes and obfuscates spans.
type Obfuscator struct{ opts *config.ObfuscationConfig }

// NewObfuscator creates a new Obfuscator.
func NewObfuscator(cfg *config.ObfuscationConfig) *Obfuscator {
	if cfg == nil {
		cfg = new(config.ObfuscationConfig)
	}
	opts := *cfg
	for _, v := range opts.ES.KeepValues {
		opts.ES.KeepMap[v] = true
	}
	for _, v := range opts.Mongo.KeepValues {
		opts.Mongo.KeepMap[v] = true
	}
	return &Obfuscator{opts: &opts}
}

// Obfuscate may obfuscate span's properties based on its type and on the Obfuscator's
// configuration.
func (o *Obfuscator) Obfuscate(span *model.Span) {
	switch span.Type {
	case "sql", "cassandra":
		o.quantizeSQL(span)
	case "redis":
		o.quantizeRedis(span)
	case "mongodb":
		o.obfuscateMongo(span)
	case "elasticsearch":
		o.obfuscateES(span)
	}
}

// compactWhitespaces compacts all whitespaces in t.
func compactWhitespaces(t string) string {
	n := len(t)
	r := make([]byte, n)
	spaceCode := uint8(32)
	isWhitespace := func(char uint8) bool { return char == spaceCode }
	nr := 0
	offset := 0
	for i := 0; i < n; i++ {
		if isWhitespace(t[i]) {
			copy(r[nr:], t[nr+offset:i])
			r[i-offset] = spaceCode
			nr = i + 1 - offset
			for j := i + 1; j < n; j++ {
				if !isWhitespace(t[j]) {
					offset += j - i - 1
					i = j
					break
				} else if j == n-1 {
					offset += j - i
					i = j
					break
				}
			}
		}
	}
	copy(r[nr:], t[nr+offset:n])

	// Trim
	rStart := 0
	rEnd := n - offset
	if isWhitespace(r[rEnd-1]) {
		rEnd--
	}
	if isWhitespace(r[rStart]) {
		rStart++
	}

	return string(r[rStart:rEnd])
}

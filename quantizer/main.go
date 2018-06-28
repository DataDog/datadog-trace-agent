// Package quantizer implements quantizing and obfuscating of tags and resources for
// a set of spans matching a certain criteria.
package quantizer

import (
	"regexp"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	tabCode     = uint8(9)
	newLineCode = uint8(10)
	spaceCode   = uint8(32)
)

var nonUniformSpacesRegexp = regexp.MustCompile("\\s+")

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

func isGenericSpace(char uint8) bool {
	return char == spaceCode || char == tabCode || char == newLineCode
}

func isWhitespace(char uint8) bool {
	return char == spaceCode
}

// compactAllSpaces transforms any sequence of space-like characters (including line breaks) into a single standard space
// Also right trims spaces.
func compactAllSpaces(t string) string {
	// Algorithm:
	//  - Iterate over the input string (of length n), char by char (cursor `i`), looking for spaces
	//  - When a spaces if found:
	//     - append a whitespace to the buffer (representing the compacted whitespace sequence)
	//     - append the preceding characters to the buffer `r`
	//     - look for the end of the whitespace sequence (cursor `j`)
	//     - resume the iteration at the end of the whitespace sequence
	//  - At the end, append the remains to the result
	//  - Trim spaces

	n := len(t)
	r := make([]byte, n)

	nr := 0     // size of the result string that we generate
	offset := 0 // number of characters removed by the compaction
	for i := 0; i < n; i++ {
		if isGenericSpace(t[i]) {
			copy(r[nr:], t[nr+offset:i])
			r[i-offset] = spaceCode
			nr = i + 1 - offset
			for j := i + 1; j < n; j++ {
				if !isGenericSpace(t[j]) {
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

// compactWhitespaces is same as compactAllSpaces, except it only apply to standard spaces
func compactWhitespaces(t string) string {
	n := len(t)
	r := make([]byte, n)

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

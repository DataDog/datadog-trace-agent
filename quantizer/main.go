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

// Quantize generates meaningful resource for a span, depending on its type
func Quantize(cfg *config.AgentConfig, span *model.Span) {
	switch span.Type {
	case "sql", "cassandra":
		QuantizeSQL(span)
	case "redis":
		QuantizeRedis(span)
	case "mongodb":
		QuantizeMongo(cfg, span)
	case "elasticsearch":
		QuantizeES(cfg, span)
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

package quantizer

import (
	"regexp"
	"strings"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

const (
	sqlType     = "sql"
	redisType   = "redis"
	tabCode     = uint8(9)
	newLineCode = uint8(10)
	spaceCode   = uint8(32)
)

var nonUniformSpacesRegexp = regexp.MustCompile("\\s+")

// Quantize generates meaningul resource for a span, depending on its type
func Quantize(span model.Span) model.Span {
	if span.Type == sqlType {
		return QuantizeSQL(span)
	} else if span.Type == redisType {
		return QuantizeRedis(span)
	}
	log.Debugf("No quantization for this span, Type: %s", span.Type)

	return span
}

// compactAllSpaces transforms any sequence of space-like characters (including line breaks) into a single standard space
// Also right trims spaces.
func compactAllSpaces(text string) string {
	return compactAllSpacesWithRegexp(text)
}

func compactAllSpacesWithRegexp(text string) string {
	return strings.TrimRight(nonUniformSpacesRegexp.ReplaceAllString(text, " "), " ")
}

func isGenericSpace(char uint8) bool {
	return char == spaceCode || char == tabCode || char == newLineCode
}

func isWhitespace(char uint8) bool {
	return char == spaceCode
}

func compactAllSpacesWithoutRegexp(t string) string {
	// Algorithm:
	//  - Iterate over the input string (of length n), char by char (cursor `i`), looking for spaces
	//  - When a spaces if found:
	//     - append a whitespace to the buffer (representing the compacted whitespace sequence)
	//     - append the preceding characters to the buffer `r`
	//     - look for the end of the whitespace sequence (cursor `j`)
	//     - resume the iteration at the end of the whitespace sequence
	//  - At the end, append the remains to the result

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

	// Rtrim
	if isWhitespace(r[n-offset-1]) {
		return string(r[0 : n-offset-1])
	} else {
		return string(r[0 : n-offset])
	}

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

	// Rtrim
	if isWhitespace(r[n-offset-1]) {
		return string(r[0 : n-offset-1])
	} else {
		return string(r[0 : n-offset])
	}
}

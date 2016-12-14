package quantizer

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
)

var r = regexp.MustCompile(`(\s*\(\s+(\?[,\s]+)+\)[\s,]*)+|\s*\[\s+(\?[,\s]+)+\]\s*`)

// TokenFilter is a generic interface that a TokenConsumer expects. It defines
// the Filter() function used to filter or replace given tokens.
type TokenFilter interface {
	Filter(token, lastToken int, buffer []byte) []byte
}

// DiscardFilter implements the TokenFilter interface so that the given
// token is discarded or accepted.
type DiscardFilter struct{}

// Filter the given token so that a `nil` slice is returned if the token
// is in the token filtered list.
func (f *DiscardFilter) Filter(token, lastToken int, buffer []byte) []byte {
	switch token {
	case Comment, ';':
		return nil
	default:
		return buffer
	}
}

// ReplaceFilter implements the TokenFilter interface so that the given
// token is replaced with '?' or left unchanged.
type ReplaceFilter struct{}

// Filter the given token so that it will be replaced if in the token replacement list
func (f *ReplaceFilter) Filter(token, lastToken int, buffer []byte) []byte {
	switch lastToken {
	case Savepoint:
		return []byte("?")
	case Limit:
		return buffer
	}

	switch token {
	case String, Number, Null, Variable, PreparedStatement, BooleanLiteral, EscapeSequence:
		return []byte("?")
	default:
		return buffer
	}
}

// TokenConsumer is a Tokenizer consumer. It calls the Tokenizer Scan() function until tokens
// are available or if a LEX_ERROR is raised. After retrieving a token, it is sent in the
// TokenFilter chains so that the token is discarded or replaced.
type TokenConsumer struct {
	tokenizer *Tokenizer
	filters   []TokenFilter
	lastToken int
}

// Process the given SQL or No-SQL string so that the resulting one is properly altered. This
// function is generic and the behavior changes according to chosen TokenFilter implementations.
// The process calls all filters inside the []TokenFilter.
func (t *TokenConsumer) Process(in string) (string, error) {
	out := &bytes.Buffer{}
	t.tokenizer.InStream.Reset(in)

	// reset the Tokenizer internals to reuse allocated memory
	defer t.tokenizer.Reset()

	token, buff := t.tokenizer.Scan()
	for ; token != EOFChar; token, buff = t.tokenizer.Scan() {
		// handle terminal case
		if token == LexError {
			// the tokenizer is unable  to process the SQL  string, so the output will be
			// surely wrong. In this case we return an error and an empty string.
			return "", errors.New("the tokenizer was unable to process the string")
		}

		// apply all registered filters
		for _, f := range t.filters {
			buff = f.Filter(token, t.lastToken, buff)
		}

		// write the resulting buffer
		if len(buff) > 0 {
			if token != ',' {
				out.WriteRune(' ')
			}

			out.Write(buff)
		}

		t.lastToken = token
	}

	// TODO[manu]: this regex has a huge downside in terms of performance because it makes
	// this execution 10x times slower than expected. This regex is used at the end to group
	// replaced token so that the output will be:
	//
	//   [...] VALUES ( ?, ?, ? ) -> VALUES ?
	//
	// This part should be replaced later to improve a lot the performance of this TokenConsumer
	// but for the moment is a good compromise.
	result := r.ReplaceAllString(out.String(), " ? ")
	result = strings.TrimSpace(result)

	return result, nil
}

// NewTokenConsumer returns a new TokenConsumer capable to process SQL and No-SQL strings.
func NewTokenConsumer(filters []TokenFilter) *TokenConsumer {
	return &TokenConsumer{
		tokenizer: NewStringTokenizer(""),
		filters:   filters,
	}
}

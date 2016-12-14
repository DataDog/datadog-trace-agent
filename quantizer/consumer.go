package quantizer

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
)

var r = regexp.MustCompile(`(\s*\(\s+(\?[,\s]+)+\)[\s,]*)+|\s*\[\s+(\?[,\s]+)+\]\s*`)

type TokenFilter interface {
	Filter(token, lastToken int, buffer []byte) []byte
}

type DiscardFilter struct{}

func (f *DiscardFilter) Filter(token, lastToken int, buffer []byte) []byte {
	switch token {
	case COMMENT, ';':
		return nil
	default:
		return buffer
	}
}

type ReplaceFilter struct{}

func (f *ReplaceFilter) Filter(token, lastToken int, buffer []byte) []byte {
	switch lastToken {
	case SAVEPOINT:
		return []byte("?")
	case LIMIT:
		return buffer
	}

	switch token {
	case STRING, NUMBER, NULL, VARIABLE, PREPARED_STATEMENT, BOOLEAN_LITERAL, ESCAPE_SEQUENCE:
		return []byte("?")
	default:
		return buffer
	}
}

type TokenConsumer struct {
	tokenizer *Tokenizer
	filters   []TokenFilter
	lastToken int
}

func (t *TokenConsumer) Process(in string) (string, error) {
	out := &bytes.Buffer{}
	t.tokenizer.InStream.Reset(in)
	token, buff := t.tokenizer.Scan()

	for ; token != EOFCHAR; token, buff = t.tokenizer.Scan() {
		// handle terminal case
		if token == LEX_ERROR {
			// TODO[manu]: in this case the tokenizer is unable  to process the SQL
			// string, so the output will be surely wrong. In this case we have to
			// decide if we want to return a partial processed string, or just an
			// error.
			return "Invalid SQL query", errors.New("the tokenizer was unable to process the string")
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

	// reset the Tokenizer internals to reuse allocated memory
	t.tokenizer.Reset()
	return result, nil
}

func NewTokenConsumer(filters []TokenFilter) *TokenConsumer {
	return &TokenConsumer{
		tokenizer: NewStringTokenizer(""),
		filters:   filters,
	}
}

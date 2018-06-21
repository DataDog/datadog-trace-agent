package quantizer

import (
	"bytes"
	"errors"

	"github.com/DataDog/datadog-trace-agent/model"
	log "github.com/cihub/seelog"
)

const (
	sqlQueryTag      = "sql.query"
	sqlQuantizeError = "agent.parse.error"
)

// TokenFilter is a generic interface that a TokenConsumer expects. It defines
// the Filter() function used to filter or replace given tokens.
// A filter can be stateful and keep an internal state to apply the filter later;
// this can be useful to prevent backtracking in some cases.
type TokenFilter interface {
	Filter(token, lastToken int, buffer []byte) (int, []byte)
	Reset()
}

// DiscardFilter implements the TokenFilter interface so that the given
// token is discarded or accepted.
type DiscardFilter struct{}

// Filter the given token so that a `nil` slice is returned if the token
// is in the token filtered list.
func (f *DiscardFilter) Filter(token, lastToken int, buffer []byte) (int, []byte) {
	// filters based on previous token
	switch lastToken {
	case As:
		// prevent the next comma from being part of a GroupingFilter
		return FilteredComma, nil
	}

	// filters based on the current token; if the next token should be ignored,
	// return the same token value (not Filtered) and nil
	switch token {
	case As:
		return As, nil
	case Comment, ';':
		return Filtered, nil
	default:
		return token, buffer
	}
}

// Reset in a DiscardFilter is a noop action
func (f *DiscardFilter) Reset() {}

// ReplaceFilter implements the TokenFilter interface so that the given
// token is replaced with '?' or left unchanged.
type ReplaceFilter struct{}

// Filter the given token so that it will be replaced if in the token replacement list
func (f *ReplaceFilter) Filter(token, lastToken int, buffer []byte) (int, []byte) {
	switch lastToken {
	case Savepoint:
		return Filtered, []byte("?")
	}
	switch token {
	case String, Number, Null, Variable, PreparedStatement, BooleanLiteral, EscapeSequence:
		return Filtered, []byte("?")
	default:
		return token, buffer
	}
}

// Reset in a ReplaceFilter is a noop action
func (f *ReplaceFilter) Reset() {}

// GroupingFilter implements the TokenFilter interface so that when
// a common pattern is identified, it's discarded to prevent duplicates
type GroupingFilter struct {
	groupFilter int
	groupMulti  int
}

// Filter the given token so that it will be discarded if a grouping pattern
// has been recognized. A grouping is composed by items like:
//   * '( ?, ?, ? )'
//   * '( ?, ? ), ( ?, ? )'
func (f *GroupingFilter) Filter(token, lastToken int, buffer []byte) (int, []byte) {
	// increasing the number of groups means that we're filtering an entire group
	// because it can be represented with a single '( ? )'
	if (lastToken == '(' && token == Filtered) || (token == '(' && f.groupMulti > 0) {
		f.groupMulti++
	}

	switch {
	case token == Filtered:
		// the previous filter has dropped this token so we should start
		// counting the group filter so that we accept only one '?' for
		// the same group
		f.groupFilter++

		if f.groupFilter > 1 {
			return Filtered, nil
		}
	case f.groupFilter > 0 && (token == ',' || token == '?'):
		// if we are in a group drop all commas
		return Filtered, nil
	case f.groupMulti > 1:
		// drop all tokens since we're in a counting group
		// and they're duplicated
		return Filtered, nil
	case token != ',' && token != '(' && token != ')' && token != Filtered:
		// when we're out of a group reset the filter state
		f.Reset()
	}

	return token, buffer
}

// Reset in a GroupingFilter restores variables used to count
// escaped token that should be filtered
func (f *GroupingFilter) Reset() {
	f.groupFilter = 0
	f.groupMulti = 0
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

	token, buff := t.tokenizer.Scan()
	for ; token != EOFChar; token, buff = t.tokenizer.Scan() {
		// handle terminal case
		if token == LexError {
			// the tokenizer is unable  to process the SQL  string, so the output will be
			// surely wrong. In this case we return an error and an empty string.
			t.Reset()
			return "", errors.New("the tokenizer was unable to process the string")
		}

		// apply all registered filters
		for _, f := range t.filters {
			token, buff = f.Filter(token, t.lastToken, buff)
		}

		// write the resulting buffer
		if buff != nil {
			// ensure that whitespaces properly separate
			// received tokens
			if out.Len() != 0 {
				switch token {
				case ',':
				case '=':
					if t.lastToken == ':' {
						break
					}
					fallthrough
				default:
					out.WriteRune(' ')
				}
			}

			out.Write(buff)
		}

		t.lastToken = token
	}

	// reset internals to reuse allocated memory
	t.Reset()
	return out.String(), nil
}

// Reset restores the initial states for all components so that memory can be re-used
func (t *TokenConsumer) Reset() {
	t.tokenizer.Reset()
	for _, f := range t.filters {
		f.Reset()
	}
}

// NewTokenConsumer returns a new TokenConsumer capable to process SQL and No-SQL strings.
func NewTokenConsumer(filters []TokenFilter) *TokenConsumer {
	return &TokenConsumer{
		tokenizer: NewStringTokenizer(""),
		filters:   filters,
	}
}

// token consumer that will quantize the query with
// the given filters; this quantizer is used only
// for SQL and CQL strings
var tokenQuantizer = NewTokenConsumer(
	[]TokenFilter{
		&DiscardFilter{},
		&ReplaceFilter{},
		&GroupingFilter{},
	})

// QuantizeSQL generates resource and sql.query meta for SQL spans
func QuantizeSQL(span *model.Span) {
	if span.Resource == "" {
		return
	}

	quantizedString, err := tokenQuantizer.Process(span.Resource)
	if err != nil || quantizedString == "" {
		// if we have an error, the partially parsed SQL is discarded so that we don't pollute
		// users resources. Here we provide more details to debug the problem.
		log.Debugf("Error parsing the query: `%s`", span.Resource)
		if span.Meta == nil {
			span.Meta = make(map[string]string)
		}
		span.Meta[sqlQuantizeError] = "Query not parsed"
		if _, ok := span.Meta[sqlQueryTag]; !ok {
			span.Meta[sqlQueryTag] = span.Resource
		}
		span.Resource = "Non-parsable SQL query"
		return
	}

	span.Resource = quantizedString

	// set the sql.query tag if and only if it's not already set by users. If a users set
	// this value, we send that value AS IS to the backend. If the value is not set, we
	// try to obfuscate users parameters so that sensitive data are not sent in the backend.
	// TODO: the current implementation is a rough approximation that assumes
	// obfuscation == quantization. This is not true in real environments because we're
	// removing data that could be interesting for users.
	if span.Meta != nil && span.Meta[sqlQueryTag] != "" {
		return
	}

	if span.Meta == nil {
		span.Meta = make(map[string]string)
	}

	span.Meta[sqlQueryTag] = quantizedString
	return
}

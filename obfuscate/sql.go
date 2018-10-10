package obfuscate

import (
	"bytes"
	"errors"

	"github.com/DataDog/datadog-trace-agent/model"
	log "github.com/cihub/seelog"
)

const sqlQueryTag = "sql.query"

// Filtered* are special token types used by the filters to identify certain states
// of parsing.
const (
	// Filtered specifies that the given token has been discarded by one of the
	// token filters.
	Filtered = 67364

	// FilteredNoGrouping specifies that the token has been discarded and should not
	// be considered for grouping.
	FilteredNoGrouping = 67366
)

// State* are special token types used by the filters to identify certain states
// of parsing and help keep tokenizing context.
const (
	// StateDiscardingBracketedAs specifies that we are currently discarding
	// a bracketed identifier (MSSQL).
	// See issue https://github.com/DataDog/datadog-trace-agent/issues/475.
	StateDiscardingBracketedAs = 67367

	// StateReplacingIn is the returned token type representing that we are in the
	// state of discarding IN parameters.
	StateReplacingIn = 67368
)

// tokenFilter is a generic interface that a sqlObfuscator expects. It defines
// the Filter() function used to filter or replace given tokens.
// A filter can be stateful and keep an internal state to apply the filter later;
// this can be useful to prevent backtracking in some cases.
type tokenFilter interface {
	Filter(token, lastToken int, buffer []byte) (int, []byte)
	Reset()
}

// discardFilter identifies tokens that need to be discarded from the output.
type discardFilter struct{}

// Filter the given token so that a `nil` slice is returned if the token
// is in the token filtered list.
func (f *discardFilter) Filter(token, lastToken int, buffer []byte) (int, []byte) {
	// filters based on previous token
	switch lastToken {
	case As:
		if token == '[' {
			// the identifier followed by AS is an MSSQL bracketed identifier
			// and will continue to be discarded until we find the corresponding
			// closing bracket counter-part. See GitHub issue #475.
			return StateDiscardingBracketedAs, nil
		}
		// prevent the next comma from being part of a groupingFilter
		return FilteredNoGrouping, nil
	case StateDiscardingBracketedAs:
		if token != ']' {
			// we haven't found the closing bracket yet, keep going
			if token != ID {
				// the token between the brackets *must* be an identifier,
				// otherwise the query is invalid.
				return LexError, nil
			}
			return StateDiscardingBracketedAs, nil
		}
		return FilteredNoGrouping, nil
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

func (f *discardFilter) Reset() {}

// replaceFilter identifies tokens that need to be replaced with '?'
type replaceFilter struct {
	// depth keeps track of how deep we are in a bracketed closure to help
	// replace paramters to IN.
	depth int
}

// Filter the given token so that it will be replaced if in the token replacement list
func (f *replaceFilter) Filter(token, lastToken int, buffer []byte) (int, []byte) {
	switch lastToken {
	case Savepoint:
		return Filtered, []byte("?")
	case In:
		if token == '(' {
			f.depth = 1
		}
		return StateReplacingIn, nil
	case StateReplacingIn:
		switch token {
		case '(':
			f.depth++
		case ')':
			f.depth--
		}
		if f.depth <= 0 {
			// done discarding IN
			return Filtered, []byte("( ? )")
		}
		return StateReplacingIn, nil
	}
	switch token {
	case String, Number, Null, Variable, PreparedStatement, BooleanLiteral, EscapeSequence:
		return Filtered, []byte("?")
	default:
		return token, buffer
	}
}

// Reset in a replaceFilter is a noop action
func (f *replaceFilter) Reset() {
	f.depth = 0
}

// groupingFilter implements the tokenFilter interface so that when
// a common pattern is identified, it's discarded to prevent duplicates
type groupingFilter struct {
	groupFilter int
	groupMulti  int
}

// Filter the given token so that it will be discarded if a grouping pattern
// has been recognized. A grouping is composed by items like:
//   * '( ?, ?, ? )'
//   * '( ?, ? ), ( ?, ? )'
func (f *groupingFilter) Filter(token, lastToken int, buffer []byte) (int, []byte) {
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

// Reset in a groupingFilter restores variables used to count
// escaped token that should be filtered
func (f *groupingFilter) Reset() {
	f.groupFilter = 0
	f.groupMulti = 0
}

// sqlObfuscator is a Tokenizer consumer. It calls the Tokenizer Scan() function until tokens
// are available or if a LEX_ERROR is raised. After retrieving a token, it is sent in the
// tokenFilter chains so that the token is discarded or replaced.
type sqlObfuscator struct {
	tokenizer *Tokenizer
	filters   []tokenFilter
	lastToken int
}

// Process the given SQL or No-SQL string so that the resulting one is properly altered. This
// function is generic and the behavior changes according to chosen tokenFilter implementations.
// The process calls all filters inside the []tokenFilter.
func (t *sqlObfuscator) obfuscate(in string) (string, error) {
	var out bytes.Buffer
	t.reset(in)
	token, buff := t.tokenizer.Scan()
	for ; token != EOFChar; token, buff = t.tokenizer.Scan() {
		if token == LexError {
			return "", errors.New("the tokenizer was unable to process the string")
		}
		for _, f := range t.filters {
			if token, buff = f.Filter(token, t.lastToken, buff); token == LexError {
				return "", errors.New("the tokenizer was unable to process the string")
			}
		}
		if buff != nil {
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
	return out.String(), nil
}

// Reset restores the initial states for all components so that memory can be re-used
func (t *sqlObfuscator) reset(in string) {
	t.tokenizer.Reset(in)
	for _, f := range t.filters {
		f.Reset()
	}
}

// newSQLObfuscator returns a new sqlObfuscator capable to process SQL and No-SQL strings.
func newSQLObfuscator() *sqlObfuscator {
	return &sqlObfuscator{
		tokenizer: NewStringTokenizer(""),
		filters: []tokenFilter{
			&discardFilter{},
			&replaceFilter{},
			&groupingFilter{},
		},
	}
}

// QuantizeSQL generates resource and sql.query meta for SQL spans
func (o *Obfuscator) obfuscateSQL(span *model.Span) {
	if span.Resource == "" {
		return
	}
	result, err := o.sql.obfuscate(span.Resource)
	if err != nil || result == "" {
		// we have an error, discard the SQL to avoid polluting user resources.
		log.Debugf("Error parsing SQL query: %q", span.Resource)
		if span.Meta == nil {
			span.Meta = make(map[string]string, 1)
		}
		if _, ok := span.Meta[sqlQueryTag]; !ok {
			span.Meta[sqlQueryTag] = span.Resource
		}
		span.Resource = "Non-parsable SQL query"
		return
	}

	span.Resource = result

	if span.Meta != nil && span.Meta[sqlQueryTag] != "" {
		// "sql.query" tag already set by user, do not change it.
		return
	}
	if span.Meta == nil {
		span.Meta = make(map[string]string)
	}
	span.Meta[sqlQueryTag] = result
}

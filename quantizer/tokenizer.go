// the tokenizer.go implementation has been inspired by https://github.com/youtube/vitess sql parser
package quantizer

import (
	"bytes"
	"fmt"
	"strings"
)

// list of available tokens; this list has been reduced because we don't
// need a full-fledged tokenizer to implement a Lexer
const (
	EOFCHAR            = 0x100
	LEX_ERROR          = 57346
	ID                 = 57347
	LIMIT              = 57348
	NULL               = 57349
	STRING             = 57350
	NUMBER             = 57351
	BOOLEAN_LITERAL    = 57352
	VALUE_ARG          = 57353
	LIST_ARG           = 57354
	COMMENT            = 57355
	VARIABLE           = 57356
	SAVEPOINT          = 57357
	PREPARED_STATEMENT = 57358
	ESCAPE_SEQUENCE    = 57359
	NULL_SAFE_EQUAL    = 57360
	LE                 = 57361
	GE                 = 57362
	NE                 = 57363
)

// Tokenizer is the struct used to generate SQL
// tokens for the parser.
type Tokenizer struct {
	InStream    *strings.Reader
	Position    int
	lastChar    uint16
	posVarIndex int
}

// NewStringTokenizer creates a new Tokenizer for the
// sql string.
func NewStringTokenizer(sql string) *Tokenizer {
	return &Tokenizer{InStream: strings.NewReader(sql)}
}

// Reset the underlying buffer and positions
func (tkn *Tokenizer) Reset() {
	tkn.InStream.Reset("")
	tkn.Position = 0
	tkn.lastChar = 0
	tkn.posVarIndex = 0
}

// keywords used to recognize string tokens
var keywords = map[string]int{
	"NULL":      NULL,
	"TRUE":      BOOLEAN_LITERAL,
	"FALSE":     BOOLEAN_LITERAL,
	"SAVEPOINT": SAVEPOINT,
	"LIMIT":     LIMIT,
}

// Scan scans the tokenizer for the next token and returns
// the token type and the token buffer.
// TODO[manu]: the current implementation returns a new Buffer
// for each Scan(). An improvement to reduce the overhead of
// the Scan() is to return slices instead of buffers.
func (tkn *Tokenizer) Scan() (int, []byte) {
	if tkn.lastChar == 0 {
		tkn.next()
	}
	tkn.skipBlank()

	switch ch := tkn.lastChar; {
	case isLetter(ch):
		return tkn.scanIdentifier()
	case isDigit(ch):
		return tkn.scanNumber(false)
	case ch == ':':
		return tkn.scanBindVar()
	default:
		tkn.next()
		switch ch {
		case EOFCHAR:
			return EOFCHAR, nil
		case '=', ',', ';', '(', ')', '+', '*', '&', '|', '^', '~', '[', ']':
			return int(ch), []byte{byte(ch)}
		case '?':
			tkn.posVarIndex++
			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, ":v%d", tkn.posVarIndex)
			return VALUE_ARG, buf.Bytes()
		case '.':
			if isDigit(tkn.lastChar) {
				return tkn.scanNumber(true)
			} else {
				return int(ch), []byte{byte(ch)}
			}
		case '/':
			switch tkn.lastChar {
			case '/':
				tkn.next()
				return tkn.scanCommentType1("//")
			case '*':
				tkn.next()
				return tkn.scanCommentType2()
			default:
				return int(ch), []byte{byte(ch)}
			}
		case '-':
			if tkn.lastChar == '-' {
				tkn.next()
				return tkn.scanCommentType1("--")
			} else {
				return int(ch), []byte{byte(ch)}
			}
		case '<':
			switch tkn.lastChar {
			case '>':
				tkn.next()
				return NE, []byte("<>")
			case '=':
				tkn.next()
				switch tkn.lastChar {
				case '>':
					tkn.next()
					return NULL_SAFE_EQUAL, []byte("<=>")
				default:
					return LE, []byte("<=")
				}
			default:
				return int(ch), []byte{byte(ch)}
			}
		case '>':
			if tkn.lastChar == '=' {
				tkn.next()
				return GE, []byte(">=")
			} else {
				return int(ch), []byte{byte(ch)}
			}
		case '!':
			if tkn.lastChar == '=' {
				tkn.next()
				return NE, []byte("!=")
			}
			return LEX_ERROR, []byte("!")
		case '\'':
			return tkn.scanString(ch, STRING)
		case '`':
			return tkn.scanLiteralIdentifier('`')
		case '"':
			return tkn.scanLiteralIdentifier('"')
		case '%':
			if tkn.lastChar == '(' {
				return tkn.scanVariableIdentifier('%')
			} else {
				return tkn.scanFormatParameter('%')
			}
		case '$':
			return tkn.scanPreparedStatement('$')
		case '{':
			return tkn.scanEscapeSequence('{')
		default:
			return LEX_ERROR, []byte{byte(ch)}
		}
	}
}

func (tkn *Tokenizer) skipBlank() {
	ch := tkn.lastChar
	for ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t' {
		tkn.next()
		ch = tkn.lastChar
	}
}

func (tkn *Tokenizer) scanIdentifier() (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteByte(byte(tkn.lastChar))
	tkn.next()

	for isLetter(tkn.lastChar) || isDigit(tkn.lastChar) || tkn.lastChar == '.' || tkn.lastChar == '*' {
		buffer.WriteByte(byte(tkn.lastChar))
		tkn.next()
	}
	lowered := bytes.ToUpper(buffer.Bytes())
	if keywordId, found := keywords[string(lowered)]; found {
		return keywordId, lowered
	}
	return ID, buffer.Bytes()
}

func (tkn *Tokenizer) scanLiteralIdentifier(quote rune) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteByte(byte(tkn.lastChar))
	if !isLetter(tkn.lastChar) {
		return LEX_ERROR, buffer.Bytes()
	}
	for tkn.next(); isLetter(tkn.lastChar) || isDigit(tkn.lastChar); tkn.next() {
		buffer.WriteByte(byte(tkn.lastChar))
	}

	// literals identifier are enclosed between quotes
	if tkn.lastChar != uint16(quote) {
		return LEX_ERROR, buffer.Bytes()
	}
	tkn.next()
	return ID, buffer.Bytes()
}

func (tkn *Tokenizer) scanVariableIdentifier(prefix rune) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteRune(prefix)
	buffer.WriteByte(byte(tkn.lastChar))

	// expects that the variable is enclosed between '(' and ')' parenthesis
	if tkn.lastChar != '(' {
		return LEX_ERROR, buffer.Bytes()
	}
	for tkn.next(); tkn.lastChar != ')'; tkn.next() {
		buffer.WriteByte(byte(tkn.lastChar))
	}

	buffer.WriteByte(byte(tkn.lastChar))
	tkn.next()
	buffer.WriteByte(byte(tkn.lastChar))
	if !isLetter(tkn.lastChar) {
		return LEX_ERROR, buffer.Bytes()
	}
	tkn.next()
	return VARIABLE, buffer.Bytes()
}

func (tkn *Tokenizer) scanFormatParameter(prefix rune) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteRune(prefix)
	buffer.WriteByte(byte(tkn.lastChar))

	// a format parameter is like '%s' so it should be a letter otherwise
	// we're having something different
	if !isLetter(tkn.lastChar) {
		return LEX_ERROR, buffer.Bytes()
	}

	tkn.next()
	return VARIABLE, buffer.Bytes()
}

func (tkn *Tokenizer) scanPreparedStatement(prefix rune) (int, []byte) {
	buffer := &bytes.Buffer{}

	// a prepared statement expect a digit identifier like $1
	if !isDigit(tkn.lastChar) {
		return LEX_ERROR, buffer.Bytes()
	}

	// read numbers and return an error if any
	token, buff := tkn.scanNumber(false)
	if token == LEX_ERROR {
		return LEX_ERROR, buffer.Bytes()
	}

	buffer.WriteRune(prefix)
	buffer.Write(buff)
	return PREPARED_STATEMENT, buffer.Bytes()
}

func (tkn *Tokenizer) scanEscapeSequence(braces rune) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteByte(byte(braces))

	for tkn.lastChar != '}' && tkn.lastChar != EOFCHAR {
		buffer.WriteByte(byte(tkn.lastChar))
		tkn.next()
	}

	// we've reached the end of the string without finding
	// the closing curly braces
	if tkn.lastChar == EOFCHAR {
		return LEX_ERROR, buffer.Bytes()
	}

	buffer.WriteByte(byte(tkn.lastChar))
	tkn.next()
	return ESCAPE_SEQUENCE, buffer.Bytes()
}

func (tkn *Tokenizer) scanBindVar() (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteByte(byte(tkn.lastChar))
	token := VALUE_ARG
	tkn.next()
	if tkn.lastChar == ':' {
		token = LIST_ARG
		buffer.WriteByte(byte(tkn.lastChar))
		tkn.next()
	}
	if !isLetter(tkn.lastChar) {
		return LEX_ERROR, buffer.Bytes()
	}
	for isLetter(tkn.lastChar) || isDigit(tkn.lastChar) || tkn.lastChar == '.' {
		buffer.WriteByte(byte(tkn.lastChar))
		tkn.next()
	}
	return token, buffer.Bytes()
}

func (tkn *Tokenizer) scanMantissa(base int, buffer *bytes.Buffer) {
	for digitVal(tkn.lastChar) < base {
		tkn.ConsumeNext(buffer)
	}
}

func (tkn *Tokenizer) scanNumber(seenDecimalPoint bool) (int, []byte) {
	buffer := &bytes.Buffer{}
	if seenDecimalPoint {
		buffer.WriteByte('.')
		tkn.scanMantissa(10, buffer)
		goto exponent
	}

	if tkn.lastChar == '0' {
		// int or float
		tkn.ConsumeNext(buffer)
		if tkn.lastChar == 'x' || tkn.lastChar == 'X' {
			// hexadecimal int
			tkn.ConsumeNext(buffer)
			tkn.scanMantissa(16, buffer)
		} else {
			// octal int or float
			seenDecimalDigit := false
			tkn.scanMantissa(8, buffer)
			if tkn.lastChar == '8' || tkn.lastChar == '9' {
				// illegal octal int or float
				seenDecimalDigit = true
				tkn.scanMantissa(10, buffer)
			}
			if tkn.lastChar == '.' || tkn.lastChar == 'e' || tkn.lastChar == 'E' {
				goto fraction
			}
			// octal int
			if seenDecimalDigit {
				return LEX_ERROR, buffer.Bytes()
			}
		}
		goto exit
	}

	// decimal int or float
	tkn.scanMantissa(10, buffer)

fraction:
	if tkn.lastChar == '.' {
		tkn.ConsumeNext(buffer)
		tkn.scanMantissa(10, buffer)
	}

exponent:
	if tkn.lastChar == 'e' || tkn.lastChar == 'E' {
		tkn.ConsumeNext(buffer)
		if tkn.lastChar == '+' || tkn.lastChar == '-' {
			tkn.ConsumeNext(buffer)
		}
		tkn.scanMantissa(10, buffer)
	}

exit:
	return NUMBER, buffer.Bytes()
}

func (tkn *Tokenizer) scanString(delim uint16, typ int) (int, []byte) {
	buffer := &bytes.Buffer{}
	for {
		ch := tkn.lastChar
		tkn.next()
		if ch == delim {
			if tkn.lastChar == delim {
				tkn.next()
			} else {
				break
			}
		} else if ch == '\\' {
			if tkn.lastChar == EOFCHAR {
				return LEX_ERROR, buffer.Bytes()
			}

			ch = tkn.lastChar
			tkn.next()
		}
		if ch == EOFCHAR {
			return LEX_ERROR, buffer.Bytes()
		}
		buffer.WriteByte(byte(ch))
	}
	return typ, buffer.Bytes()
}

func (tkn *Tokenizer) scanCommentType1(prefix string) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteString(prefix)
	for tkn.lastChar != EOFCHAR {
		if tkn.lastChar == '\n' {
			tkn.ConsumeNext(buffer)
			break
		}
		tkn.ConsumeNext(buffer)
	}
	return COMMENT, buffer.Bytes()
}

func (tkn *Tokenizer) scanCommentType2() (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteString("/*")
	for {
		if tkn.lastChar == '*' {
			tkn.ConsumeNext(buffer)
			if tkn.lastChar == '/' {
				tkn.ConsumeNext(buffer)
				break
			}
			continue
		}
		if tkn.lastChar == EOFCHAR {
			return LEX_ERROR, buffer.Bytes()
		}
		tkn.ConsumeNext(buffer)
	}
	return COMMENT, buffer.Bytes()
}

func (tkn *Tokenizer) ConsumeNext(buffer *bytes.Buffer) {
	if tkn.lastChar == EOFCHAR {
		// This should never happen.
		panic("unexpected EOF")
	}
	buffer.WriteByte(byte(tkn.lastChar))
	tkn.next()
}

func (tkn *Tokenizer) next() {
	if ch, err := tkn.InStream.ReadByte(); err != nil {
		// Only EOF is possible.
		tkn.lastChar = EOFCHAR
	} else {
		tkn.lastChar = uint16(ch)
	}
	tkn.Position++
}

func isLetter(ch uint16) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch == '@' || ch == '#'
}

func digitVal(ch uint16) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch) - '0'
	case 'a' <= ch && ch <= 'f':
		return int(ch) - 'a' + 10
	case 'A' <= ch && ch <= 'F':
		return int(ch) - 'A' + 10
	}
	return 16 // larger than any legal digit val
}

func isDigit(ch uint16) bool {
	return '0' <= ch && ch <= '9'
}

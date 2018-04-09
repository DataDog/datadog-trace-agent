package quantizer

import (
	"bytes"
	"strings"
)

// tokenizer.go implemenents a lexer-like iterator that tokenizes SQL and CQL
// strings, so that an external component can filter or alter each token of the
// string. This implementation can't be used as a real SQL lexer (so a parser
// cannot build the AST) because many rules are ignored to make the tokenizer
// simpler.
// This implementation was inspired by https://github.com/youtube/vitess sql parser
// TODO: add the license to the NOTICE file

// list of available tokens; this list has been reduced because we don't
// need a full-fledged tokenizer to implement a Lexer
const (
	EOFChar           = 0x100
	LexError          = 57346
	ID                = 57347
	Limit             = 57348
	Null              = 57349
	String            = 57350
	Number            = 57351
	BooleanLiteral    = 57352
	ValueArg          = 57353
	ListArg           = 57354
	Comment           = 57355
	Variable          = 57356
	Savepoint         = 57357
	PreparedStatement = 57358
	EscapeSequence    = 57359
	NullSafeEqual     = 57360
	LE                = 57361
	GE                = 57362
	NE                = 57363
	As                = 57365
	Select            = 57367
	Set               = 57368
	ReservedKeyword   = 57369

	// Filtered specifies that the given token has been discarded by one of the
	// token filters.
	Filtered = 57364

	// FilteredComma specifies that the token is a comma and was discarded by one
	// of the filters.
	FilteredComma = 57366
)

// Tokenizer is the struct used to generate SQL
// tokens for the parser.
type Tokenizer struct {
	InStream *strings.Reader
	Position int
	lastChar uint16
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
}

// keywords used to recognize string tokens
var keywords = map[string]int{
	"NULL":                Null,
	"TRUE":                BooleanLiteral,
	"FALSE":               BooleanLiteral,
	"SAVEPOINT":           Savepoint,
	"LIMIT":               Limit,
	"AS":                  As,
	"SELECT":              Select,
	"SET":                 Set,
	"ACCESSIBLE":          ReservedKeyword,
	"ADD":                 ReservedKeyword,
	"AGAINST":             ReservedKeyword,
	"ALL":                 ReservedKeyword,
	"ALTER":               ReservedKeyword,
	"ANALYZE":             ReservedKeyword,
	"AND":                 ReservedKeyword,
	"ASC":                 ReservedKeyword,
	"ASENSITIVE":          ReservedKeyword,
	"AUTO_INCREMENT":      ReservedKeyword,
	"BEFORE":              ReservedKeyword,
	"BEGIN":               ReservedKeyword,
	"BETWEEN":             ReservedKeyword,
	"BIGINT":              ReservedKeyword,
	"BINARY":              ReservedKeyword,
	"_BINARY":             ReservedKeyword,
	"BIT":                 ReservedKeyword,
	"BLOB":                ReservedKeyword,
	"BOOL":                ReservedKeyword,
	"BOOLEAN":             ReservedKeyword,
	"BOTH":                ReservedKeyword,
	"BY":                  ReservedKeyword,
	"CALL":                ReservedKeyword,
	"CASCADE":             ReservedKeyword,
	"CASE":                ReservedKeyword,
	"CAST":                ReservedKeyword,
	"CHANGE":              ReservedKeyword,
	"CHAR":                ReservedKeyword,
	"CHARACTER":           ReservedKeyword,
	"CHARSET":             ReservedKeyword,
	"COLLATE":             ReservedKeyword,
	"COLUMN":              ReservedKeyword,
	"COMMENT":             ReservedKeyword,
	"COMMIT":              ReservedKeyword,
	"CONDITION":           ReservedKeyword,
	"CONSTRAINT":          ReservedKeyword,
	"CONTINUE":            ReservedKeyword,
	"CONVERT":             ReservedKeyword,
	"SUBSTR":              ReservedKeyword,
	"SUBSTRING":           ReservedKeyword,
	"CREATE":              ReservedKeyword,
	"CROSS":               ReservedKeyword,
	"CURRENT_DATE":        ReservedKeyword,
	"CURRENT_TIME":        ReservedKeyword,
	"CURRENT_TIMESTAMP":   ReservedKeyword,
	"CURRENT_USER":        ReservedKeyword,
	"CURSOR":              ReservedKeyword,
	"DATABASE":            ReservedKeyword,
	"DATABASES":           ReservedKeyword,
	"DAY_HOUR":            ReservedKeyword,
	"DAY_MICROSECOND":     ReservedKeyword,
	"DAY_MINUTE":          ReservedKeyword,
	"DAY_SECOND":          ReservedKeyword,
	"DATETIME":            ReservedKeyword,
	"DEC":                 ReservedKeyword,
	"DECIMAL":             ReservedKeyword,
	"DECLARE":             ReservedKeyword,
	"DEFAULT":             ReservedKeyword,
	"DELAYED":             ReservedKeyword,
	"DELETE":              ReservedKeyword,
	"DESC":                ReservedKeyword,
	"DESCRIBE":            ReservedKeyword,
	"DETERMINISTIC":       ReservedKeyword,
	"DISTINCT":            ReservedKeyword,
	"DISTINCTROW":         ReservedKeyword,
	"DIV":                 ReservedKeyword,
	"DOUBLE":              ReservedKeyword,
	"DROP":                ReservedKeyword,
	"DUPLICATE":           ReservedKeyword,
	"EACH":                ReservedKeyword,
	"ELSE":                ReservedKeyword,
	"ELSEIF":              ReservedKeyword,
	"ENCLOSED":            ReservedKeyword,
	"END":                 ReservedKeyword,
	"ENUM":                ReservedKeyword,
	"ESCAPE":              ReservedKeyword,
	"ESCAPED":             ReservedKeyword,
	"EXISTS":              ReservedKeyword,
	"EXIT":                ReservedKeyword,
	"EXPLAIN":             ReservedKeyword,
	"EXPANSION":           ReservedKeyword,
	"FETCH":               ReservedKeyword,
	"FLOAT":               ReservedKeyword,
	"FLOAT4":              ReservedKeyword,
	"FLOAT8":              ReservedKeyword,
	"FOR":                 ReservedKeyword,
	"FORCE":               ReservedKeyword,
	"FOREIGN":             ReservedKeyword,
	"FROM":                ReservedKeyword,
	"FULLTEXT":            ReservedKeyword,
	"GENERATED":           ReservedKeyword,
	"GET":                 ReservedKeyword,
	"GLOBAL":              ReservedKeyword,
	"GRANT":               ReservedKeyword,
	"GROUP":               ReservedKeyword,
	"GROUP_CONCAT":        ReservedKeyword,
	"HAVING":              ReservedKeyword,
	"HIGH_PRIORITY":       ReservedKeyword,
	"HOUR_MICROSECOND":    ReservedKeyword,
	"HOUR_MINUTE":         ReservedKeyword,
	"HOUR_SECOND":         ReservedKeyword,
	"IF":                  ReservedKeyword,
	"IGNORE":              ReservedKeyword,
	"IN":                  ReservedKeyword,
	"INDEX":               ReservedKeyword,
	"INFILE":              ReservedKeyword,
	"INOUT":               ReservedKeyword,
	"INNER":               ReservedKeyword,
	"INSENSITIVE":         ReservedKeyword,
	"INSERT":              ReservedKeyword,
	"INT":                 ReservedKeyword,
	"INT1":                ReservedKeyword,
	"INT2":                ReservedKeyword,
	"INT3":                ReservedKeyword,
	"INT4":                ReservedKeyword,
	"INT8":                ReservedKeyword,
	"INTEGER":             ReservedKeyword,
	"INTERVAL":            ReservedKeyword,
	"INTO":                ReservedKeyword,
	"IO_AFTER_GTIDS":      ReservedKeyword,
	"IS":                  ReservedKeyword,
	"ITERATE":             ReservedKeyword,
	"JOIN":                ReservedKeyword,
	"JSON":                ReservedKeyword,
	"KEYS":                ReservedKeyword,
	"KILL":                ReservedKeyword,
	"LANGUAGE":            ReservedKeyword,
	"LAST_INSERT_ID":      ReservedKeyword,
	"LEADING":             ReservedKeyword,
	"LEAVE":               ReservedKeyword,
	"LEFT":                ReservedKeyword,
	"LESS":                ReservedKeyword,
	"LIKE":                ReservedKeyword,
	"LINEAR":              ReservedKeyword,
	"LINES":               ReservedKeyword,
	"LOAD":                ReservedKeyword,
	"LOCALTIME":           ReservedKeyword,
	"LOCALTIMESTAMP":      ReservedKeyword,
	"LOCK":                ReservedKeyword,
	"LONG":                ReservedKeyword,
	"LONGBLOB":            ReservedKeyword,
	"LONGTEXT":            ReservedKeyword,
	"LOOP":                ReservedKeyword,
	"LOW_PRIORITY":        ReservedKeyword,
	"MASTER_BIND":         ReservedKeyword,
	"MATCH":               ReservedKeyword,
	"MAXVALUE":            ReservedKeyword,
	"MEDIUMBLOB":          ReservedKeyword,
	"MEDIUMINT":           ReservedKeyword,
	"MEDIUMTEXT":          ReservedKeyword,
	"MIDDLEINT":           ReservedKeyword,
	"MINUTE_MICROSECOND":  ReservedKeyword,
	"MINUTE_SECOND":       ReservedKeyword,
	"MOD":                 ReservedKeyword,
	"MODE":                ReservedKeyword,
	"MODIFIES":            ReservedKeyword,
	"NAMES":               ReservedKeyword,
	"NATURAL":             ReservedKeyword,
	"NCHAR":               ReservedKeyword,
	"NEXT":                ReservedKeyword,
	"NOT":                 ReservedKeyword,
	"NO_WRITE_TO_BINLOG":  ReservedKeyword,
	"NUMERIC":             ReservedKeyword,
	"OFFSET":              ReservedKeyword,
	"ON":                  ReservedKeyword,
	"OPTIMIZE":            ReservedKeyword,
	"OPTIMIZER_COSTS":     ReservedKeyword,
	"OPTION":              ReservedKeyword,
	"OPTIONALLY":          ReservedKeyword,
	"OR":                  ReservedKeyword,
	"ORDER":               ReservedKeyword,
	"OUT":                 ReservedKeyword,
	"OUTER":               ReservedKeyword,
	"OUTFILE":             ReservedKeyword,
	"PARTITION":           ReservedKeyword,
	"PRECISION":           ReservedKeyword,
	"PRIMARY":             ReservedKeyword,
	"PROCEDURE":           ReservedKeyword,
	"QUERY":               ReservedKeyword,
	"RANGE":               ReservedKeyword,
	"READ":                ReservedKeyword,
	"READS":               ReservedKeyword,
	"READ_WRITE":          ReservedKeyword,
	"REAL":                ReservedKeyword,
	"REFERENCES":          ReservedKeyword,
	"REGEXP":              ReservedKeyword,
	"RELEASE":             ReservedKeyword,
	"RENAME":              ReservedKeyword,
	"REORGANIZE":          ReservedKeyword,
	"REPAIR":              ReservedKeyword,
	"REPEAT":              ReservedKeyword,
	"REPLACE":             ReservedKeyword,
	"REQUIRE":             ReservedKeyword,
	"RESIGNAL":            ReservedKeyword,
	"RESTRICT":            ReservedKeyword,
	"RETURN":              ReservedKeyword,
	"REVOKE":              ReservedKeyword,
	"RIGHT":               ReservedKeyword,
	"RLIKE":               ReservedKeyword,
	"ROLLBACK":            ReservedKeyword,
	"SCHEMA":              ReservedKeyword,
	"SCHEMAS":             ReservedKeyword,
	"SECOND_MICROSECOND":  ReservedKeyword,
	"SENSITIVE":           ReservedKeyword,
	"SEPARATOR":           ReservedKeyword,
	"SESSION":             ReservedKeyword,
	"SHARE":               ReservedKeyword,
	"SHOW":                ReservedKeyword,
	"SIGNAL":              ReservedKeyword,
	"SIGNED":              ReservedKeyword,
	"SMALLINT":            ReservedKeyword,
	"SPATIAL":             ReservedKeyword,
	"SPECIFIC":            ReservedKeyword,
	"SQL":                 ReservedKeyword,
	"SQLEXCEPTION":        ReservedKeyword,
	"SQLSTATE":            ReservedKeyword,
	"SQLWARNING":          ReservedKeyword,
	"SQL_BIG_RESULT":      ReservedKeyword,
	"SQL_CACHE":           ReservedKeyword,
	"SQL_CALC_FOUND_ROWS": ReservedKeyword,
	"SQL_NO_CACHE":        ReservedKeyword,
	"SQL_SMALL_RESULT":    ReservedKeyword,
	"SSL":                 ReservedKeyword,
	"START":               ReservedKeyword,
	"STARTING":            ReservedKeyword,
	"STORED":              ReservedKeyword,
	"STRAIGHT_JOIN":       ReservedKeyword,
	"STREAM":              ReservedKeyword,
	"TABLE":               ReservedKeyword,
	"TABLES":              ReservedKeyword,
	"TERMINATED":          ReservedKeyword,
	"TEXT":                ReservedKeyword,
	"THAN":                ReservedKeyword,
	"THEN":                ReservedKeyword,
	"TIME":                ReservedKeyword,
	"TINYBLOB":            ReservedKeyword,
	"TINYINT":             ReservedKeyword,
	"TINYTEXT":            ReservedKeyword,
	"TO":                  ReservedKeyword,
	"TRAILING":            ReservedKeyword,
	"TRANSACTION":         ReservedKeyword,
	"TRIGGER":             ReservedKeyword,
	"TRUNCATE":            ReservedKeyword,
	"UNDO":                ReservedKeyword,
	"UNION":               ReservedKeyword,
	"UNIQUE":              ReservedKeyword,
	"UNLOCK":              ReservedKeyword,
	"UNSIGNED":            ReservedKeyword,
	"UPDATE":              ReservedKeyword,
	"USAGE":               ReservedKeyword,
	"USE":                 ReservedKeyword,
	"USING":               ReservedKeyword,
	"UTC_DATE":            ReservedKeyword,
	"UTC_TIME":            ReservedKeyword,
	"UTC_TIMESTAMP":       ReservedKeyword,
	"VALUES":              ReservedKeyword,
	"VARIABLES":           ReservedKeyword,
	"VARBINARY":           ReservedKeyword,
	"VARCHAR":             ReservedKeyword,
	"VARCHARACTER":        ReservedKeyword,
	"VARYING":             ReservedKeyword,
	"VIRTUAL":             ReservedKeyword,
	"VINDEX":              ReservedKeyword,
	"VINDEXES":            ReservedKeyword,
	"VIEW":                ReservedKeyword,
	"VITESS_KEYSPACES":    ReservedKeyword,
	"VITESS_SHARDS":       ReservedKeyword,
	"VITESS_TABLETS":      ReservedKeyword,
	"VSCHEMA_TABLES":      ReservedKeyword,
	"WHEN":                ReservedKeyword,
	"WHERE":               ReservedKeyword,
	"WHILE":               ReservedKeyword,
	"WITH":                ReservedKeyword,
	"WRITE":               ReservedKeyword,
	"XOR":                 ReservedKeyword,
	"YEAR":                ReservedKeyword,
	"YEAR_MONTH":          ReservedKeyword,
	"ZEROFILL":            ReservedKeyword,
}

// Scan scans the tokenizer for the next token and returns
// the token type and the token buffer.
func (tkn *Tokenizer) Scan() (int, []byte) {
	if tkn.lastChar == 0 {
		tkn.next()
	}
	tkn.skipBlank()

	switch ch := tkn.lastChar; {
	case isLeadingLetter(ch):
		return tkn.scanIdentifier()
	case isDigit(ch):
		return tkn.scanNumber(false)
	case ch == ':':
		return tkn.scanBindVar()
	default:
		tkn.next()
		switch ch {
		case EOFChar:
			return EOFChar, nil
		case '=', ',', ';', '(', ')', '+', '*', '&', '|', '^', '~', '[', ']', '?':
			return int(ch), []byte{byte(ch)}
		case '.':
			if isDigit(tkn.lastChar) {
				return tkn.scanNumber(true)
			}
			return int(ch), []byte{byte(ch)}
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
			}
			return int(ch), []byte{byte(ch)}
		case '#':
			tkn.next()
			return tkn.scanCommentType1("#")
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
					return NullSafeEqual, []byte("<=>")
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
			}
			return int(ch), []byte{byte(ch)}
		case '!':
			if tkn.lastChar == '=' {
				tkn.next()
				return NE, []byte("!=")
			}
			return LexError, []byte("!")
		case '\'':
			return tkn.scanString(ch, String)
		case '`':
			return tkn.scanLiteralIdentifier('`')
		case '"':
			return tkn.scanLiteralIdentifier('"')
		case '%':
			if tkn.lastChar == '(' {
				return tkn.scanVariableIdentifier('%')
			}
			return tkn.scanFormatParameter('%')
		case '$':
			return tkn.scanPreparedStatement('$')
		case '{':
			return tkn.scanEscapeSequence('{')
		default:
			return LexError, []byte{byte(ch)}
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
	upper := bytes.ToUpper(buffer.Bytes())
	if keywordID, found := keywords[string(upper)]; found {
		return keywordID, upper
	}
	return ID, buffer.Bytes()
}

func (tkn *Tokenizer) scanLiteralIdentifier(quote rune) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteByte(byte(tkn.lastChar))
	if !isLetter(tkn.lastChar) {
		return LexError, buffer.Bytes()
	}
	for tkn.next(); skipNonLiteralIdentifier(tkn.lastChar); tkn.next() {
		buffer.WriteByte(byte(tkn.lastChar))
	}
	// literals identifier are enclosed between quotes
	if tkn.lastChar != uint16(quote) {
		return LexError, buffer.Bytes()
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
		return LexError, buffer.Bytes()
	}
	for tkn.next(); tkn.lastChar != ')' && tkn.lastChar != EOFChar; tkn.next() {
		buffer.WriteByte(byte(tkn.lastChar))
	}

	buffer.WriteByte(byte(tkn.lastChar))
	tkn.next()
	buffer.WriteByte(byte(tkn.lastChar))
	if !isLetter(tkn.lastChar) {
		return LexError, buffer.Bytes()
	}
	tkn.next()
	return Variable, buffer.Bytes()
}

func (tkn *Tokenizer) scanFormatParameter(prefix rune) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteRune(prefix)
	buffer.WriteByte(byte(tkn.lastChar))

	// a format parameter is like '%s' so it should be a letter otherwise
	// we're having something different
	if !isLetter(tkn.lastChar) {
		return LexError, buffer.Bytes()
	}

	tkn.next()
	return Variable, buffer.Bytes()
}

func (tkn *Tokenizer) scanPreparedStatement(prefix rune) (int, []byte) {
	buffer := &bytes.Buffer{}

	// a prepared statement expect a digit identifier like $1
	if !isDigit(tkn.lastChar) {
		return LexError, buffer.Bytes()
	}

	// read numbers and return an error if any
	token, buff := tkn.scanNumber(false)
	if token == LexError {
		return LexError, buffer.Bytes()
	}

	buffer.WriteRune(prefix)
	buffer.Write(buff)
	return PreparedStatement, buffer.Bytes()
}

func (tkn *Tokenizer) scanEscapeSequence(braces rune) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteByte(byte(braces))

	for tkn.lastChar != '}' && tkn.lastChar != EOFChar {
		buffer.WriteByte(byte(tkn.lastChar))
		tkn.next()
	}

	// we've reached the end of the string without finding
	// the closing curly braces
	if tkn.lastChar == EOFChar {
		return LexError, buffer.Bytes()
	}

	buffer.WriteByte(byte(tkn.lastChar))
	tkn.next()
	return EscapeSequence, buffer.Bytes()
}

func (tkn *Tokenizer) scanBindVar() (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteByte(byte(tkn.lastChar))
	token := ValueArg
	tkn.next()
	if tkn.lastChar == ':' {
		token = ListArg
		buffer.WriteByte(byte(tkn.lastChar))
		tkn.next()
	}
	if !isLetter(tkn.lastChar) {
		return LexError, buffer.Bytes()
	}
	for isLetter(tkn.lastChar) || isDigit(tkn.lastChar) || tkn.lastChar == '.' {
		buffer.WriteByte(byte(tkn.lastChar))
		tkn.next()
	}
	return token, buffer.Bytes()
}

func (tkn *Tokenizer) scanMantissa(base int, buffer *bytes.Buffer) {
	for digitVal(tkn.lastChar) < base {
		tkn.consumeNext(buffer)
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
		tkn.consumeNext(buffer)
		if tkn.lastChar == 'x' || tkn.lastChar == 'X' {
			// hexadecimal int
			tkn.consumeNext(buffer)
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
				return LexError, buffer.Bytes()
			}
		}
		goto exit
	}

	// decimal int or float
	tkn.scanMantissa(10, buffer)

fraction:
	if tkn.lastChar == '.' {
		tkn.consumeNext(buffer)
		tkn.scanMantissa(10, buffer)
	}

exponent:
	if tkn.lastChar == 'e' || tkn.lastChar == 'E' {
		tkn.consumeNext(buffer)
		if tkn.lastChar == '+' || tkn.lastChar == '-' {
			tkn.consumeNext(buffer)
		}
		tkn.scanMantissa(10, buffer)
	}

exit:
	return Number, buffer.Bytes()
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
			if tkn.lastChar == EOFChar {
				return LexError, buffer.Bytes()
			}

			ch = tkn.lastChar
			tkn.next()
		}
		if ch == EOFChar {
			return LexError, buffer.Bytes()
		}
		buffer.WriteByte(byte(ch))
	}
	return typ, buffer.Bytes()
}

func (tkn *Tokenizer) scanCommentType1(prefix string) (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteString(prefix)
	for tkn.lastChar != EOFChar {
		if tkn.lastChar == '\n' {
			tkn.consumeNext(buffer)
			break
		}
		tkn.consumeNext(buffer)
	}
	return Comment, buffer.Bytes()
}

func (tkn *Tokenizer) scanCommentType2() (int, []byte) {
	buffer := &bytes.Buffer{}
	buffer.WriteString("/*")
	for {
		if tkn.lastChar == '*' {
			tkn.consumeNext(buffer)
			if tkn.lastChar == '/' {
				tkn.consumeNext(buffer)
				break
			}
			continue
		}
		if tkn.lastChar == EOFChar {
			return LexError, buffer.Bytes()
		}
		tkn.consumeNext(buffer)
	}
	return Comment, buffer.Bytes()
}

func (tkn *Tokenizer) consumeNext(buffer *bytes.Buffer) {
	if tkn.lastChar == EOFChar {
		// This should never happen.
		panic("unexpected EOF")
	}
	buffer.WriteByte(byte(tkn.lastChar))
	tkn.next()
}

func (tkn *Tokenizer) next() {
	if ch, err := tkn.InStream.ReadByte(); err != nil {
		// Only EOF is possible.
		tkn.lastChar = EOFChar
	} else {
		tkn.lastChar = uint16(ch)
	}
	tkn.Position++
}

func skipNonLiteralIdentifier(ch uint16) bool {
	return isLetter(ch) || isDigit(ch) || '.' == ch || '-' == ch
}

func isLeadingLetter(ch uint16) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch == '@'
}

func isLetter(ch uint16) bool {
	return isLeadingLetter(ch) || ch == '#'
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

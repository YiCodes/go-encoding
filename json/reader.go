package json

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

type Reader struct {
	reader      *bufio.Reader
	peekedToken jsonToken
	char        rune
	charPeeked  bool
	row         int
	col         int
	offset      int
}

type position struct {
	offset int
	col    int
	row    int
}

func (p *position) String() string {
	return fmt.Sprintf("[%v, %v]", p.row, p.col)
}

func NewReader(reader io.Reader) *Reader {
	jsonReader := &Reader{}
	jsonReader.reader = bufio.NewReader(reader)

	return jsonReader
}

type tokenKind int

const (
	kindEOF tokenKind = iota
	kindString
	kindNumber
	kindTrue
	kindFalse
	kindNull
	kindLeftBrace
	kindRightBrace
	kindLeftBracket
	kindRightBracket
	kindColon
	kindComma
	kindBadToken
)

func (k tokenKind) String() string {
	switch k {
	case kindEOF:
		return "EOF"
	case kindString:
		return "String"
	case kindNumber:
		return "Number"
	case kindTrue:
		return "true"
	case kindFalse:
		return "false"
	case kindNull:
		return "null"
	case kindLeftBrace:
		return "{"
	case kindRightBrace:
		return "}"
	case kindLeftBracket:
		return "["
	case kindRightBracket:
		return "]"
	case kindColon:
		return ":"
	case kindComma:
		return ","
	}

	return ""
}

type jsonToken interface {
	Kind() tokenKind
	Pos(pos *position) *position
	String() string
}

type jsonBasicToken struct {
	kind     tokenKind
	startPos position
	value    string
}

func (t *jsonBasicToken) Kind() tokenKind {
	return t.kind
}

func (t *jsonBasicToken) Pos(pos *position) *position {
	if pos != nil {
		t.startPos = *pos
	}

	return &t.startPos
}

func (t *jsonBasicToken) String() string {
	return t.value
}

type jsonStringToken struct {
	jsonBasicToken

	str string
}

func (j *Reader) readChar() (rune, error) {
	if j.charPeeked {
		j.charPeeked = false
	} else {
		r, _, err := j.reader.ReadRune()

		if err != nil {
			return r, err
		}

		j.char = r
	}

	j.offset++
	return j.char, nil
}

func (j *Reader) peekChar() (rune, bool) {
	r, _, err := j.reader.ReadRune()

	if err == nil {
		j.charPeeked = true
		return r, true
	}

	return r, false
}

func (j *Reader) nextToken() jsonToken {
	if j.peekedToken != nil {
		t := j.peekedToken
		j.peekedToken = nil
		return t
	}

	for {
		pos := j.getPos()
		c, ok := j.peekChar()

		if !ok {
			return j.newBasicToken(kindEOF, "", pos)
		}

		if c >= '0' && c <= '9' {
			return j.readNumberToken()
		} else if c == '"' {
			return j.readStringToken()
		} else if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return j.readKeywordToken()
		} else if unicode.IsSpace(c) {
			j.readChar()
			continue
		}

		c, _ = j.readChar()

		switch c {
		case '{':
			return j.newBasicToken(kindLeftBrace, "{", pos)
		case '}':
			return j.newBasicToken(kindRightBrace, "}", pos)
		case '[':
			return j.newBasicToken(kindLeftBracket, "[", pos)
		case ']':
			return j.newBasicToken(kindRightBracket, "]", pos)
		case ':':
			return j.newBasicToken(kindColon, ":", pos)
		case ',':
			return j.newBasicToken(kindComma, ",", pos)
		default:
			return j.newBasicToken(kindBadToken, string(c), pos)
		}
	}
}

func (j *Reader) getPos() *position {
	return &position{offset: j.offset, row: j.row, col: j.col}
}

func (j *Reader) readKeywordToken() jsonToken {
	var b strings.Builder

	pos := j.getPos()

	for {
		c, ok := j.peekChar()

		if ok && ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			j.readChar()
			b.WriteRune(c)
		}

		break
	}

	value := b.String()
	var kind tokenKind

	if value == "true" {
		kind = kindTrue
	} else if value == "false" {
		kind = kindFalse
	} else if value == "null" {
		kind = kindNull
	}

	return j.newBasicToken(kind, value, pos)
}

func (j *Reader) readStringToken() jsonToken {
	var b strings.Builder
	var escape bool

	pos := j.getPos()

	j.readChar()
	j.col++

	for {
		c, ok := j.peekChar()

		if !ok {
			return j.newBasicToken(kindBadToken, b.String(), pos)
		}

		j.col++

		if escape {
			escape = false
		} else if c == '"' {
			break
		} else if c == '\\' {
			escape = true
		} else if c == '\r' || c == '\n' {
			return j.newBasicToken(kindBadToken, b.String(), pos)
		}

		j.readChar()

		b.WriteRune(c)
	}

	t := &jsonStringToken{}
	t.kind = kindString
	t.str = b.String()
	t.value = fmt.Sprintf(`"%v"`, t.str)
	t.startPos = *pos
	return t
}

func (j *Reader) newBasicToken(kind tokenKind, value string, pos *position) jsonToken {
	t := &jsonBasicToken{
		kind:     kind,
		value:    value,
		startPos: *pos,
	}

	return t
}

func (j *Reader) readNumberToken() jsonToken {
	var b strings.Builder
	isInt := true

	pos := j.getPos()

	for {
		c, ok := j.peekChar()

		if !ok {
			break
		}

		if c == '.' && isInt {
			isInt = false
			fmt.Fprint(&b, c)
		} else if c >= '0' && c <= '9' {
			fmt.Fprint(&b, c)
		} else {
			break
		}

		j.readChar()
		j.col++
	}

	return j.newBasicToken(kindNumber, b.String(), pos)
}

func (j *Reader) peekToken() jsonToken {
	if j.peekedToken == nil {
		j.peekedToken = j.nextToken()
	}

	return j.peekedToken
}

func (j *Reader) eat(kind tokenKind) (jsonToken, error) {
	t := j.nextToken()

	if t.Kind() == kind {
		return t, nil
	}

	return t, fmt.Errorf("expect %v %v", kind.String(), t.Pos(nil).String())
}

func (j *Reader) mayEat(kind tokenKind) bool {
	t := j.peekToken()

	return t.Kind() == kind
}

func (j *Reader) TryReadNull() bool {
	return j.mayEat(kindNull)
}

func (j *Reader) ReadStartObject() error {
	_, err := j.eat(kindLeftBrace)

	return err
}

func (j *Reader) ReadEndObject() error {
	_, err := j.eat(kindRightBrace)

	return err
}

func (j *Reader) TryReadEndObject() bool {
	return j.mayEat(kindRightBrace)
}

func (j *Reader) ReadStartArray() error {
	_, err := j.eat(kindLeftBracket)

	return err
}

func (j *Reader) ReadEndArray() error {
	_, err := j.eat(kindRightBracket)

	return err
}

func (j *Reader) TryReadEndArray() bool {
	return j.mayEat(kindRightBracket)
}

func (j *Reader) ReadStartField(fieldName string) error {
	t, err := j.eat(kindString)

	if err != nil {
		return err
	}

	if strToken := t.(*jsonStringToken); strToken.str != fieldName {
		return fmt.Errorf("except fieldName is %v, but %v", fieldName, strToken.str)
	}

	j.eat(kindColon)

	return nil
}

func (j *Reader) ReadString() (string, error) {
	if j.mayEat(kindNull) {
		return "", nil
	}

	t, err := j.eat(kindString)

	if err != nil {
		return "", err
	}

	strTok := t.(*jsonStringToken)

	return strTok.str, nil
}

func (j *Reader) ReadInt() (int, error) {
	t, err := j.eat(kindNumber)

	if err != nil {
		return 0, err
	}

	return strconv.Atoi(t.String())
}

func (j *Reader) ReadInt64() (int64, error) {
	t, err := j.eat(kindNumber)

	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(t.String(), 0, 64)
}

func (j *Reader) ReadFloat() (float64, error) {
	t, err := j.eat(kindNumber)

	if err != nil {
		return 0, err
	}

	return strconv.ParseFloat(t.String(), 64)
}

func (j *Reader) ReadBool() (bool, error) {
	if j.mayEat(kindTrue) {
		return false, nil
	} else if j.mayEat(kindFalse) {
		return false, nil
	}

	t := j.nextToken()

	return false, fmt.Errorf("expect bool value %v", t.Pos(nil).String())
}

func (j *Reader) ReadEndField() {
	j.mayEat(kindComma)
}

package lexer

import (
	"fmt"
	"strings"
)

type TokenType int

const (
	// Special
	ILLEGAL TokenType = iota
	EOF
	WS // Whitespace

	// Literals
	IDENTIFIER // table_name, column_name
	STRING     // 'value'
	NUMBER     // 123, 1.23

	// Keywords
	SELECT
	FROM
	WHERE
	INSERT
	INTO
	VALUES
	UPDATE
	SET
	DELETE
	AND
	OR
	TRUE
	FALSE
	JOIN
	INNER
	LEFT
	RIGHT
	FULL
	OUTER
	ON
	DATE
	TIME
	EMAIL

	// Operators & Punctuation
	ASTERISK    // *
	COMMA       // ,
	PAREN_OPEN  // (
	PAREN_CLOSE // )
	EQUALS      // =
	LESS_THAN   // <
	GREATER_THAN // >
	LESS_EQUAL   // <=
	GREATER_EQUAL // >=
	NOT_EQUAL    // != or <>
	DOT          // .
	SEMICOLON   // ;
)

var keywords = map[string]TokenType{
	"SELECT": SELECT,
	"FROM":   FROM,
	"WHERE":  WHERE,
	"INSERT": INSERT,
	"INTO":   INTO,
	"VALUES": VALUES,
	"UPDATE": UPDATE,
	"SET":    SET,
	"DELETE": DELETE,
	"AND":    AND,
	"OR":     OR,
	"TRUE":   TRUE,
	"FALSE":  FALSE,
	"JOIN":   JOIN,
	"INNER":  INNER,
	"LEFT":   LEFT,
	"RIGHT":  RIGHT,
	"FULL":   FULL,
	"OUTER":  OUTER,
	"ON":     ON,
	"DATE":   DATE,
	"TIME":   TIME,
	"EMAIL":  EMAIL,
}

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, %q)", t.Type, t.Literal)
}

type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
	line         int
	column       int
}

func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, column: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition += 1
	l.column++
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	tok.Line = l.line
	tok.Column = l.column

	switch l.ch {
	case '*':
		tok = newToken(ASTERISK, l.ch, l.line, l.column)
	case ',':
		tok = newToken(COMMA, l.ch, l.line, l.column)
	case '(':
		tok = newToken(PAREN_OPEN, l.ch, l.line, l.column)
	case ')':
		tok = newToken(PAREN_CLOSE, l.ch, l.line, l.column)
	case '=':
		tok = newToken(EQUALS, l.ch, l.line, l.column)
	case '<':
		// Check for <= or <>
		if l.peekChar() == '=' {
			ch := l.ch
			col := l.column
			l.readChar()
			tok = Token{Type: LESS_EQUAL, Literal: string(ch) + string(l.ch), Line: l.line, Column: col}
		} else if l.peekChar() == '>' {
			ch := l.ch
			col := l.column
			l.readChar()
			tok = Token{Type: NOT_EQUAL, Literal: string(ch) + string(l.ch), Line: l.line, Column: col}
		} else {
			tok = newToken(LESS_THAN, l.ch, l.line, l.column)
		}
	case '>':
		// Check for >=
		if l.peekChar() == '=' {
			ch := l.ch
			col := l.column
			l.readChar()
			tok = Token{Type: GREATER_EQUAL, Literal: string(ch) + string(l.ch), Line: l.line, Column: col}
		} else {
			tok = newToken(GREATER_THAN, l.ch, l.line, l.column)
		}
	case '!':
		// Check for !=
		if l.peekChar() == '=' {
			ch := l.ch
			col := l.column
			l.readChar()
			tok = Token{Type: NOT_EQUAL, Literal: string(ch) + string(l.ch), Line: l.line, Column: col}
		} else {
			// ! by itself is illegal in SQL
			tok = newToken(ILLEGAL, l.ch, l.line, l.column)
		}
	case '.':
		tok = newToken(DOT, l.ch, l.line, l.column)
	case ';':
		tok = newToken(SEMICOLON, l.ch, l.line, l.column)
	case '\'':
		tok.Type = STRING
		tok.Literal = l.readString()
		return tok
	case 0:
		tok.Literal = ""
		tok.Type = EOF
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = LookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			tok.Type = NUMBER
			tok.Literal = l.readNumber()
			return tok
		} else {
			tok = newToken(ILLEGAL, l.ch, l.line, l.column)
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		if l.ch == '\n' {
			l.line++
			l.column = 0
		}
		l.readChar()
	}
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	// Support simple floats
	if l.ch == '.' && isDigit(l.peekChar()) {
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	return l.input[position:l.position]
}

func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '\'' || l.ch == 0 {
			break
		}
	}
	lit := l.input[position:l.position]
	
	// Consume the closing quote
	if l.ch == '\'' {
		l.readChar()
	}
	
	return lit
}

func newToken(tokenType TokenType, ch byte, line, col int) Token {
	return Token{Type: tokenType, Literal: string(ch), Line: line, Column: col}
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[strings.ToUpper(ident)]; ok {
		return tok
	}
	return IDENTIFIER
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// Helper to tokenize entire string at once
func Tokenize(input string) ([]Token, error) {
	l := New(input)
	var tokens []Token
	for {
		tok := l.NextToken()
		if tok.Type == EOF {
			break
		}
		if tok.Type == ILLEGAL {
			return nil, fmt.Errorf("illegal token at line %d, col %d: %s", tok.Line, tok.Column, tok.Literal)
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

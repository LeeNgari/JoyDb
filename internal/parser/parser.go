package parser

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// Converts tokens into an ast
type Parser struct {
	tokens  []lexer.Token // List of tokens from the lexer
	curPos  int           // Current position in the token list
	curTok  lexer.Token   // Current token being examined
	peekTok lexer.Token   // Next token
}

func New(tokens []lexer.Token) *Parser {
	p := &Parser{tokens: tokens, curPos: 0}
	// Read two tokens to set curTok and peekTok
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curTok = p.peekTok
	if p.curPos < len(p.tokens) {
		p.peekTok = p.tokens[p.curPos]
		p.curPos++
	} else {
		p.peekTok = lexer.Token{Type: lexer.EOF}
	}
}

func (p *Parser) Parse() (ast.Statement, error) {
	switch p.curTok.Type {
	case lexer.SELECT:
		return p.parseSelect()
	case lexer.INSERT:
		return p.parseInsert()
	case lexer.UPDATE:
		return p.parseUpdate()
	case lexer.DELETE:
		return p.parseDelete()
	case lexer.CREATE:
		return p.parseCreate()
	case lexer.DROP:
		return p.parseDrop()
	case lexer.ALTER:
		return p.parseAlter()
	case lexer.USE:
		return p.parseUse()
	default:
		return nil, fmt.Errorf("unexpected token %v, expected a valid SQL statement (SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, USE)", p.curTok.Type)
	}
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekTok.Type == t {
		p.nextToken()
		return true
	}
	return false
}

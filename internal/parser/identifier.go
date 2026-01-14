package parser

import (
	"fmt"
	"strings"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseIdentifierList parses a comma-separated list of identifiers
// Handles both SELECT field lists and INSERT column lists
// Supports qualified identifiers (table.column) and wildcards (*)
func (p *Parser) parseIdentifierList() ([]*ast.Identifier, error) {
	var identifiers []*ast.Identifier

	// Handle first identifier or *
	if p.curTok.Type == lexer.ASTERISK {
		identifiers = append(identifiers, &ast.Identifier{TokenLiteralValue: "*", Value: "*"})
		p.nextToken()
		return identifiers, nil
	}

	// Handle ( for column list in INSERT
	if p.curTok.Type == lexer.PAREN_OPEN {
		p.nextToken()
	}

	// Parse first identifier (could be IDENTIFIER or keyword like EMAIL/DATE/TIME)
	if !isIdentifierOrKeyword(p.curTok.Type) {
		return nil, fmt.Errorf("expected identifier, got %s", p.curTok.Literal)
	}

	// Parse first identifier (possibly qualified or keyword)
	ident, err := p.parseQualifiedIdentifier()
	if err != nil {
		return nil, err
	}
	identifiers = append(identifiers, ident)

	// Parse remaining identifiers
	for p.curTok.Type == lexer.COMMA {
		p.nextToken()
		if p.curTok.Type != lexer.IDENTIFIER && p.curTok.Type != lexer.EMAIL && 
		   p.curTok.Type != lexer.DATE && p.curTok.Type != lexer.TIME {
			return nil, fmt.Errorf("expected identifier after comma, got %s", p.curTok.Literal)
		}
		ident, err := p.parseQualifiedIdentifier()
		if err != nil {
			return nil, err
		}
		identifiers = append(identifiers, ident)
	}

	// Handle ) for column list in INSERT
	if p.curTok.Type == lexer.PAREN_CLOSE {
		p.nextToken()
	}

	return identifiers, nil
}

// parseQualifiedIdentifier parses an identifier that may be qualified (table.column)
// or unqualified (column). Used in SELECT field lists and other contexts.
// Also handles EMAIL, DATE, TIME keywords when used as column names.
func (p *Parser) parseQualifiedIdentifier() (*ast.Identifier, error) {
	// Accept IDENTIFIER or keywords (EMAIL, DATE, TIME) as column names
	if !isIdentifierOrKeyword(p.curTok.Type) {
		return nil, fmt.Errorf("expected identifier, got %s", p.curTok.Literal)
	}

	firstPart := strings.ToLower(p.curTok.Literal)
	p.nextToken()

	// Check for qualified identifier (table.column)
	if p.curTok.Type == lexer.DOT {
		p.nextToken()
		if p.curTok.Type != lexer.IDENTIFIER && p.curTok.Type != lexer.EMAIL && 
		   p.curTok.Type != lexer.DATE && p.curTok.Type != lexer.TIME {
			return nil, fmt.Errorf("expected column name after '.', got %s", p.curTok.Literal)
		}
		colName := strings.ToLower(p.curTok.Literal)
		p.nextToken()
		return &ast.Identifier{
			TokenLiteralValue: firstPart + "." + colName,
			Table:             firstPart,
			Value:             colName,
		}, nil
	}

	// Unqualified identifier
	return &ast.Identifier{TokenLiteralValue: firstPart, Value: firstPart}, nil
}

// parseExpressionList parses a comma-separated list of expressions
// Used in INSERT VALUES clause and function arguments
func (p *Parser) parseExpressionList() ([]ast.Expression, error) {
	var list []ast.Expression

	if p.curTok.Type == lexer.PAREN_OPEN {
		p.nextToken()
	}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	list = append(list, expr)

	for p.curTok.Type == lexer.COMMA {
		p.nextToken()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		list = append(list, expr)
	}

	if p.curTok.Type == lexer.PAREN_CLOSE {
		p.nextToken()
	}

	return list, nil
}

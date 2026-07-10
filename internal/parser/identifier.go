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

// parseSelectFields parses a comma-separated list of select fields
// which can be either Identifiers or AggregateFunctionCalls
func (p *Parser) parseSelectFields() ([]ast.Expression, error) {
	var fields []ast.Expression

	// Handle first field
	field, err := p.parseSelectField()
	if err != nil {
		return nil, err
	}
	fields = append(fields, field)

	// Parse remaining fields
	for p.curTok.Type == lexer.COMMA {
		p.nextToken()
		field, err := p.parseSelectField()
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}

	return fields, nil
}

func (p *Parser) parseSelectField() (ast.Expression, error) {
	// Handle *
	if p.curTok.Type == lexer.ASTERISK {
		ident := &ast.Identifier{TokenLiteralValue: "*", Value: "*"}
		p.nextToken()
		return ident, nil
	}

	// Check for aggregate functions: COUNT, SUM, MIN, MAX, AVG
	if p.curTok.Type == lexer.COUNT || p.curTok.Type == lexer.SUM || 
	   p.curTok.Type == lexer.MIN || p.curTok.Type == lexer.MAX || 
	   p.curTok.Type == lexer.AVG {
		
		funcName := p.curTok.Literal
		p.nextToken() // consume function name

		if p.curTok.Type != lexer.PAREN_OPEN {
			return nil, fmt.Errorf("expected ( after aggregate function %s, got %s", funcName, p.curTok.Literal)
		}
		p.nextToken() // consume (

		var arg *ast.Identifier
		if p.curTok.Type == lexer.ASTERISK {
			arg = &ast.Identifier{TokenLiteralValue: "*", Value: "*"}
			p.nextToken() // consume *
		} else {
			var err error
			arg, err = p.parseQualifiedIdentifier()
			if err != nil {
				return nil, err
			}
		}

		if p.curTok.Type != lexer.PAREN_CLOSE {
			return nil, fmt.Errorf("expected ) after aggregate argument, got %s", p.curTok.Literal)
		}
		p.nextToken() // consume )

		agg := &ast.AggregateFunctionCall{
			Function: strings.ToUpper(funcName),
			Argument: arg,
		}

		// Check for AS alias on aggregate function
		// e.g. COUNT(*) AS total
		if p.curTok.Type == lexer.AS {
			p.nextToken()
			if p.curTok.Type != lexer.IDENTIFIER {
				return nil, fmt.Errorf("expected identifier after AS, got %s", p.curTok.Literal)
			}
			agg.Alias = strings.ToLower(p.curTok.Literal)
			p.nextToken()
		}

		return agg, nil
	}

	// Normal identifier
	return p.parseQualifiedIdentifier()
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

	var ident *ast.Identifier

	// Check for qualified identifier (table.column)
	if p.curTok.Type == lexer.DOT {
		p.nextToken()
		if p.curTok.Type != lexer.IDENTIFIER && p.curTok.Type != lexer.EMAIL && 
		   p.curTok.Type != lexer.DATE && p.curTok.Type != lexer.TIME {
			return nil, fmt.Errorf("expected column name after '.', got %s", p.curTok.Literal)
		}
		colName := strings.ToLower(p.curTok.Literal)
		p.nextToken()
		ident = &ast.Identifier{
			TokenLiteralValue: firstPart + "." + colName,
			Table:             firstPart,
			Value:             colName,
		}
	} else {
		// Unqualified identifier
		ident = &ast.Identifier{TokenLiteralValue: firstPart, Value: firstPart}
	}

	// Check for AS alias
	if p.curTok.Type == lexer.AS {
		p.nextToken()
		if p.curTok.Type != lexer.IDENTIFIER {
			return nil, fmt.Errorf("expected identifier after AS, got %s", p.curTok.Literal)
		}
		ident.Alias = strings.ToLower(p.curTok.Literal)
		p.nextToken()
	}

	return ident, nil
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

package parser

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseExpression parses expressions with logical operators (AND, OR) and comparisons
// Implements precedence: OR (lowest) < AND < Comparison operators (highest)
// Examples: 
//   - age > 18 AND active = true
//   - status = 'pending' OR status = 'processing'
//   - (age > 18 AND active = true) OR premium = true
func (p *Parser) parseExpression() (ast.Expression, error) {
	return p.parseOrExpression()
}

// parseOrExpression handles OR operations (lowest precedence)
func (p *Parser) parseOrExpression() (ast.Expression, error) {
	left, err := p.parseAndExpression()
	if err != nil {
		return nil, err
	}

	// Handle multiple OR operations (left-associative)
	for p.curTok.Type == lexer.OR {
		op := p.curTok.Literal
		p.nextToken()
		right, err := p.parseAndExpression()
		if err != nil {
			return nil, err
		}
		left = &ast.LogicalExpression{Left: left, Operator: op, Right: right}
	}

	return left, nil
}

// parseAndExpression handles AND operations (higher precedence than OR)
func (p *Parser) parseAndExpression() (ast.Expression, error) {
	left, err := p.parseComparisonExpression()
	if err != nil {
		return nil, err
	}

	// Handle multiple AND operations (left-associative)
	for p.curTok.Type == lexer.AND {
		op := p.curTok.Literal
		p.nextToken()
		right, err := p.parseComparisonExpression()
		if err != nil {
			return nil, err
		}
		left = &ast.LogicalExpression{Left: left, Operator: op, Right: right}
	}

	return left, nil
}

// parseComparisonExpression handles comparison operations (highest precedence)
// Supports: =, <, >, <=, >=, !=, <>
// Also handles parenthesized expressions for grouping
func (p *Parser) parseComparisonExpression() (ast.Expression, error) {
	// Handle parentheses for grouping
	if p.curTok.Type == lexer.PAREN_OPEN {
		p.nextToken()
		expr, err := p.parseExpression() // Recursive: allows nested logical expressions
		if err != nil {
			return nil, err
		}
		if p.curTok.Type != lexer.PAREN_CLOSE {
			return nil, fmt.Errorf("expected ), got %s", p.curTok.Literal)
		}
		p.nextToken()
		return expr, nil
	}

	// Parse left side (identifier or literal)
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}

	// Check for comparison operator
	if isComparisonOperator(p.curTok.Type) {
		op := p.curTok.Literal
		p.nextToken()
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		return &ast.BinaryExpression{Left: left, Operator: op, Right: right}, nil
	}

	return left, nil
}

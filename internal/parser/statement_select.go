package parser

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseSelect parses a SELECT statement
// Grammar: SELECT fields FROM table [JOIN ...] [WHERE condition]
func (p *Parser) parseSelect() (*ast.SelectStatement, error) {
	stmt := &ast.SelectStatement{}

	// SELECT keyword - already consumed by Parse()
	p.nextToken()

	// Fields
	fields, err := p.parseIdentifierList()
	if err != nil {
		return nil, err
	}
	stmt.Fields = fields

	// FROM
	if p.curTok.Type != lexer.FROM {
		return nil, fmt.Errorf("expected FROM, got %s", p.curTok.Literal)
	}
	p.nextToken()

	// Table Name
	if p.curTok.Type != lexer.IDENTIFIER {
		return nil, fmt.Errorf("expected table name, got %s", p.curTok.Literal)
	}
	stmt.TableName = &ast.Identifier{TokenLiteralValue: p.curTok.Literal, Value: p.curTok.Literal}
	p.nextToken()

	// JOINs (Optional, can have multiple)
	for isJoinKeyword(p.curTok.Type) {
		join, err := p.parseJoin()
		if err != nil {
			return nil, err
		}
		stmt.Joins = append(stmt.Joins, join)
	}

	// WHERE (Optional)
	if p.curTok.Type == lexer.WHERE {
		p.nextToken()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = expr
	}

	// Semicolon (Optional)
	if p.curTok.Type == lexer.SEMICOLON {
		p.nextToken()
	}

	return stmt, nil
}

// parseJoin parses a JOIN clause
// Grammar: [INNER|LEFT|RIGHT|FULL] [OUTER] JOIN table ON condition
// Examples:
//   - INNER JOIN orders ON users.id = orders.user_id
//   - LEFT OUTER JOIN orders ON users.id = orders.user_id
func (p *Parser) parseJoin() (*ast.JoinClause, error) {
	join := &ast.JoinClause{}

	// Determine JOIN type
	switch p.curTok.Type {
	case lexer.INNER:
		join.JoinType = "INNER"
		p.nextToken()
	case lexer.LEFT:
		join.JoinType = "LEFT"
		p.nextToken()
	case lexer.RIGHT:
		join.JoinType = "RIGHT"
		p.nextToken()
	case lexer.FULL:
		join.JoinType = "FULL"
		p.nextToken()
	case lexer.JOIN:
		// Default to INNER JOIN if no type specified
		join.JoinType = "INNER"
	default:
		return nil, fmt.Errorf("expected JOIN keyword, got %s", p.curTok.Literal)
	}

	// Optional OUTER keyword (for LEFT OUTER, RIGHT OUTER, FULL OUTER)
	if p.curTok.Type == lexer.OUTER {
		p.nextToken()
	}

	// JOIN keyword
	if p.curTok.Type != lexer.JOIN {
		return nil, fmt.Errorf("expected JOIN, got %s", p.curTok.Literal)
	}
	p.nextToken()

	// Right table name
	if p.curTok.Type != lexer.IDENTIFIER {
		return nil, fmt.Errorf("expected table name after JOIN, got %s", p.curTok.Literal)
	}
	join.RightTable = &ast.Identifier{TokenLiteralValue: p.curTok.Literal, Value: p.curTok.Literal}
	p.nextToken()

	// ON keyword
	if p.curTok.Type != lexer.ON {
		return nil, fmt.Errorf("expected ON, got %s", p.curTok.Literal)
	}
	p.nextToken()

	// ON condition (e.g., users.id = orders.user_id)
	condition, err := p.parseExpression()
	if err != nil {
		return nil, fmt.Errorf("failed to parse JOIN condition: %w", err)
	}
	join.OnCondition = condition

	return join, nil
}

// isJoinKeyword checks if the current token starts a JOIN clause
func isJoinKeyword(t lexer.TokenType) bool {
	return t == lexer.INNER || t == lexer.LEFT || t == lexer.RIGHT || t == lexer.FULL || t == lexer.JOIN
}

package parser

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseDelete parses a DELETE statement
// Grammar: DELETE FROM table_name [WHERE condition]
// Example: DELETE FROM users WHERE active = false
func (p *Parser) parseDelete() (*ast.DeleteStatement, error) {
	stmt := &ast.DeleteStatement{}

	// DELETE keyword - already consumed by Parse()
	p.nextToken()

	// FROM keyword
	if p.curTok.Type != lexer.FROM {
		return nil, fmt.Errorf("expected FROM after DELETE, got %s", p.curTok.Literal)
	}
	p.nextToken()

	// Table name
	if p.curTok.Type != lexer.IDENTIFIER {
		return nil, fmt.Errorf("expected table name after FROM, got %s", p.curTok.Literal)
	}
	stmt.TableName = &ast.Identifier{TokenLiteralValue: p.curTok.Literal, Value: p.curTok.Literal}
	p.nextToken()

	// WHERE clause (optional)
	if p.curTok.Type == lexer.WHERE {
		p.nextToken()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, fmt.Errorf("failed to parse WHERE clause: %w", err)
		}
		stmt.Where = expr
	}

	// Semicolon (optional)
	if p.curTok.Type == lexer.SEMICOLON {
		p.nextToken()
	}

	return stmt, nil
}

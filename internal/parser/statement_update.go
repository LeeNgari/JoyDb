package parser

import (
	"fmt"
	"strings"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseUpdate parses an UPDATE statement
// Grammar: UPDATE table_name SET col1 = val1, col2 = val2 [WHERE condition]
// Example: UPDATE users SET email = 'new@test.com', active = true WHERE id = 5
func (p *Parser) parseUpdate() (*ast.UpdateStatement, error) {
	stmt := &ast.UpdateStatement{
		Updates: make(map[string]ast.Expression),
	}

	// UPDATE keyword - already consumed by Parse()
	p.nextToken()

	// Table name
	if p.curTok.Type != lexer.IDENTIFIER {
		return nil, fmt.Errorf("expected table name after UPDATE, got %s", p.curTok.Literal)
	}
	stmt.TableName = &ast.Identifier{TokenLiteralValue: p.curTok.Literal, Value: p.curTok.Literal}
	p.nextToken()

	// SET keyword
	if p.curTok.Type != lexer.SET {
		return nil, fmt.Errorf("expected SET, got %s", p.curTok.Literal)
	}
	p.nextToken()

	// Parse SET assignments (col = val, col2 = val2, ...)
	for {
		// Column name (can be IDENTIFIER or keywords like EMAIL, DATE, TIME)
		var colName string
		if p.curTok.Type == lexer.IDENTIFIER {
			colName = p.curTok.Literal
		} else if isTypedLiteralKeyword(p.curTok.Type) {
			// Allow EMAIL, DATE, TIME as column names
			colName = strings.ToLower(p.curTok.Literal)
		} else {
			return nil, fmt.Errorf("expected column name in SET clause, got %s", p.curTok.Literal)
		}
		p.nextToken()

		// Equals sign
		if p.curTok.Type != lexer.EQUALS {
			return nil, fmt.Errorf("expected = after column name, got %s", p.curTok.Literal)
		}
		p.nextToken()

		// Value (literal)
		val, err := p.parseAtom()
		if err != nil {
			return nil, fmt.Errorf("failed to parse value in SET clause: %w", err)
		}
		lit, ok := val.(*ast.Literal)
		if !ok {
			return nil, fmt.Errorf("expected literal value in SET clause")
		}
		stmt.Updates[colName] = lit

		// Check for comma (more updates) or end of SET clause
		if p.curTok.Type == lexer.COMMA {
			p.nextToken()
			continue // Parse next column = value pair
		}

		// No comma, so we're done with SET clause
		break
	}

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

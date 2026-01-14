package parser

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseInsert parses an INSERT statement
// Grammar: INSERT INTO table (columns) VALUES (values)
func (p *Parser) parseInsert() (*ast.InsertStatement, error) {
	stmt := &ast.InsertStatement{}

	// INSERT keyword - already consumed by Parse()
	p.nextToken()

	// INTO
	if p.curTok.Type != lexer.INTO {
		return nil, fmt.Errorf("expected INTO, got %s", p.curTok.Literal)
	}
	p.nextToken()

	// Table Name
	if p.curTok.Type != lexer.IDENTIFIER {
		return nil, fmt.Errorf("expected table name, got %s", p.curTok.Literal)
	}
	stmt.TableName = &ast.Identifier{TokenLiteralValue: p.curTok.Literal, Value: p.curTok.Literal}
	p.nextToken()

	// Columns (Optional but we'll require them for now or handle parens)
	if p.curTok.Type == lexer.PAREN_OPEN {
		// Parse columns
		cols, err := p.parseIdentifierList()
		if err != nil {
			return nil, err
		}
		stmt.Columns = cols
	}

	// VALUES
	if p.curTok.Type != lexer.VALUES {
		return nil, fmt.Errorf("expected VALUES, got %s", p.curTok.Literal)
	}
	p.nextToken()

	// (
	if p.curTok.Type != lexer.PAREN_OPEN {
		return nil, fmt.Errorf("expected (, got %s", p.curTok.Literal)
	}
	
	// Parse Values List
	values, err := p.parseExpressionList()
	if err != nil {
		return nil, err
	}
	stmt.Values = values

	// Semicolon (Optional)
	if p.curTok.Type == lexer.SEMICOLON {
		p.nextToken()
	}

	return stmt, nil
}

package parser

import (
	"fmt"

	"github.com/leengari/joydb/internal/parser/ast"
	"github.com/leengari/joydb/internal/parser/lexer"
)

// parseSelect parses a SELECT statement
// Grammar: SELECT fields FROM table [JOIN ...] [WHERE condition]
func (p *Parser) parseSelect() (*ast.SelectStatement, error) {
	stmt := &ast.SelectStatement{}

	// SELECT keyword - already consumed by Parse()
	p.nextToken()

	// Fields
	fields, err := p.parseSelectFields()
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

	// Check for table alias
	if p.curTok.Type == lexer.AS {
		p.nextToken()
		if p.curTok.Type != lexer.IDENTIFIER {
			return nil, fmt.Errorf("expected identifier after AS, got %s", p.curTok.Literal)
		}
		stmt.TableAlias = p.curTok.Literal
		p.nextToken()
	}

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

	// GROUP BY (Optional)
	if p.curTok.Type == lexer.GROUP {
		p.nextToken()
		if p.curTok.Type != lexer.BY {
			return nil, fmt.Errorf("expected BY after GROUP, got %s", p.curTok.Literal)
		}
		p.nextToken()

		for {
			column, err := p.parseQualifiedIdentifier()
			if err != nil {
				return nil, fmt.Errorf("expected column name after GROUP BY: %w", err)
			}
			stmt.GroupBy = append(stmt.GroupBy, column)
			if p.curTok.Type != lexer.COMMA {
				break
			}
			p.nextToken()
		}
	}

	// ORDER BY (Optional)
	if p.curTok.Type == lexer.ORDER {
		p.nextToken()
		if p.curTok.Type != lexer.BY {
			return nil, fmt.Errorf("expected BY after ORDER, got %s", p.curTok.Literal)
		}
		p.nextToken()

		var orderBy []ast.OrderByClause
		for {
			if p.curTok.Type != lexer.IDENTIFIER {
				return nil, fmt.Errorf("expected column name after ORDER BY, got %s", p.curTok.Literal)
			}

			col := &ast.Identifier{TokenLiteralValue: p.curTok.Literal, Value: p.curTok.Literal}
			p.nextToken()

			// Check for table qualification (table.column)
			if p.curTok.Type == lexer.DOT {
				p.nextToken()
				if p.curTok.Type != lexer.IDENTIFIER {
					return nil, fmt.Errorf("expected column name after dot, got %s", p.curTok.Literal)
				}
				col.Table = col.Value
				col.Value = p.curTok.Literal
				col.TokenLiteralValue = col.Table + "." + col.Value
				p.nextToken()
			}

			desc := false
			if p.curTok.Type == lexer.ASC {
				p.nextToken()
			} else if p.curTok.Type == lexer.DESC {
				desc = true
				p.nextToken()
			}

			orderBy = append(orderBy, ast.OrderByClause{
				Column: col,
				Desc:   desc,
			})

			if p.curTok.Type == lexer.COMMA {
				p.nextToken()
				continue
			}
			break
		}
		stmt.OrderBy = orderBy
	}

	// LIMIT (Optional)
	if p.curTok.Type == lexer.LIMIT {
		p.nextToken()
		if p.curTok.Type != lexer.NUMBER {
			return nil, fmt.Errorf("expected number after LIMIT, got %s", p.curTok.Literal)
		}
		var limit int
		if _, err := fmt.Sscanf(p.curTok.Literal, "%d", &limit); err != nil {
			return nil, fmt.Errorf("invalid LIMIT value: %s", p.curTok.Literal)
		}
		stmt.Limit = &limit
		p.nextToken()
	}

	// OFFSET (Optional)
	if p.curTok.Type == lexer.OFFSET {
		p.nextToken()
		if p.curTok.Type != lexer.NUMBER {
			return nil, fmt.Errorf("expected number after OFFSET, got %s", p.curTok.Literal)
		}
		var offset int
		if _, err := fmt.Sscanf(p.curTok.Literal, "%d", &offset); err != nil {
			return nil, fmt.Errorf("invalid OFFSET value: %s", p.curTok.Literal)
		}
		stmt.Offset = &offset
		p.nextToken()
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

	// Check for right table alias
	if p.curTok.Type == lexer.AS {
		p.nextToken()
		if p.curTok.Type != lexer.IDENTIFIER {
			return nil, fmt.Errorf("expected identifier after AS, got %s", p.curTok.Literal)
		}
		join.RightTableAlias = p.curTok.Literal
		p.nextToken()
	}

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

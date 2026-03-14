package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
	"github.com/leengari/mini-rdbms/internal/validation"
)

// parseAtom parses atomic expressions (identifiers, literals, typed literals)
// This is the lowest level of expression parsing
func (p *Parser) parseAtom() (ast.Expression, error) {
	switch p.curTok.Type {
	case lexer.IDENTIFIER:
		val := p.curTok.Literal
		p.nextToken()
		
		// Check for qualified identifier (table.column)
		if p.curTok.Type == lexer.DOT {
			p.nextToken()
			if p.curTok.Type != lexer.IDENTIFIER {
				return nil, fmt.Errorf("expected column name after '.', got %s", p.curTok.Literal)
			}
			colName := p.curTok.Literal
			p.nextToken()
			return &ast.Identifier{
				TokenLiteralValue: val + "." + colName,
				Table:             val,
				Value:             colName,
			}, nil
		}
		
		// Unqualified identifier
		return &ast.Identifier{TokenLiteralValue: val, Value: val}, nil
	
	// Allow EMAIL, DATE, TIME as column names when not used as typed literals
	case lexer.EMAIL, lexer.DATE, lexer.TIME:
		// Peek ahead - if next token is STRING, this is a typed literal
		// Otherwise, treat it as an identifier (column name)
		keywordType := p.curTok.Type
		keyword := p.curTok.Literal
		
		// Check if this is a typed literal (keyword followed by string)
		// by checking the next token
		p.nextToken()
		
		if p.curTok.Type == lexer.STRING {
			// This is a typed literal - we need to parse it properly
			// Put back the keyword token and call parseTypedLiteral
			switch keywordType {
			case lexer.DATE:
				// Validate the string value
				value := p.curTok.Literal
				if err := validation.ValidateDate(value); err != nil {
					return nil, fmt.Errorf("DATE validation failed: %w", err)
				}
				p.nextToken()
				return &ast.Literal{
					TokenLiteralValue: "DATE '" + value + "'",
					Value:             value,
					Kind:              ast.LiteralDate,
				}, nil
			case lexer.TIME:
				value := p.curTok.Literal
				if err := validation.ValidateTime(value); err != nil {
					return nil, fmt.Errorf("TIME validation failed: %w", err)
				}
				p.nextToken()
				return &ast.Literal{
					TokenLiteralValue: "TIME '" + value + "'",
					Value:             value,
					Kind:              ast.LiteralTime,
				}, nil
			case lexer.EMAIL:
				value := p.curTok.Literal
				if err := validation.ValidateEmail(value); err != nil {
					return nil, fmt.Errorf("EMAIL validation failed: %w", err)
				}
				p.nextToken()
				return &ast.Literal{
					TokenLiteralValue: "EMAIL '" + value + "'",
					Value:             value,
					Kind:              ast.LiteralEmail,
				}, nil
			}
		}
		
		// Not followed by STRING, treat as identifier (column name)
		// p.curTok is already at the next token, so don't advance
		return &ast.Identifier{
			TokenLiteralValue: strings.ToLower(keyword),
			Value:             strings.ToLower(keyword),
		}, nil
	case lexer.STRING:
		val := p.curTok.Literal
		p.nextToken()
		return &ast.Literal{TokenLiteralValue: val, Value: val, Kind: ast.LiteralString}, nil
	case lexer.NUMBER:
		valStr := p.curTok.Literal
		p.nextToken()
		// Try int
		if i, err := strconv.Atoi(valStr); err == nil {
			return &ast.Literal{TokenLiteralValue: valStr, Value: i, Kind: ast.LiteralInt}, nil
		}
		// Try float
		if f, err := strconv.ParseFloat(valStr, 64); err == nil {
			return &ast.Literal{TokenLiteralValue: valStr, Value: f, Kind: ast.LiteralFloat}, nil
		}
		return nil, fmt.Errorf("invalid number: %s", valStr)
	case lexer.TRUE:
		p.nextToken()
		return &ast.Literal{TokenLiteralValue: "true", Value: true, Kind: ast.LiteralBool}, nil
	case lexer.FALSE:
		p.nextToken()
		return &ast.Literal{TokenLiteralValue: "false", Value: false, Kind: ast.LiteralBool}, nil
	default:
		return nil, fmt.Errorf("unexpected token in expression: %s", p.curTok.Literal)
	}
}


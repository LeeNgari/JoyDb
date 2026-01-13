package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

type Parser struct {
	tokens  []lexer.Token
	curPos  int
	curTok  lexer.Token
	peekTok lexer.Token
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
	default:
		return nil, fmt.Errorf("unexpected token %v, expected SELECT, INSERT, UPDATE, or DELETE", p.curTok.Type)
	}
}

func (p *Parser) parseSelect() (*ast.SelectStatement, error) {
	stmt := &ast.SelectStatement{}

	// SELECT
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

func (p *Parser) parseInsert() (*ast.InsertStatement, error) {
	stmt := &ast.InsertStatement{}

	// INSERT
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
		} else if p.curTok.Type == lexer.EMAIL || p.curTok.Type == lexer.DATE || p.curTok.Type == lexer.TIME {
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

	if p.curTok.Type != lexer.IDENTIFIER {
		return nil, fmt.Errorf("expected identifier, got %s", p.curTok.Literal)
	}

	// Parse first identifier (possibly qualified)
	ident, err := p.parseQualifiedIdentifier()
	if err != nil {
		return nil, err
	}
	identifiers = append(identifiers, ident)

	// Parse remaining identifiers
	for p.curTok.Type == lexer.COMMA {
		p.nextToken()
		if p.curTok.Type != lexer.IDENTIFIER {
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
func (p *Parser) parseQualifiedIdentifier() (*ast.Identifier, error) {
	if p.curTok.Type != lexer.IDENTIFIER {
		return nil, fmt.Errorf("expected identifier, got %s", p.curTok.Literal)
	}

	firstPart := p.curTok.Literal
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
			TokenLiteralValue: firstPart + "." + colName,
			Table:             firstPart,
			Value:             colName,
		}, nil
	}

	// Unqualified identifier
	return &ast.Identifier{TokenLiteralValue: firstPart, Value: firstPart}, nil
}

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

// isComparisonOperator checks if a token type is a comparison operator
func isComparisonOperator(t lexer.TokenType) bool {
	return t == lexer.EQUALS ||
		t == lexer.LESS_THAN ||
		t == lexer.GREATER_THAN ||
		t == lexer.LESS_EQUAL ||
		t == lexer.GREATER_EQUAL ||
		t == lexer.NOT_EQUAL
}

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
				if err := validateDate(value); err != nil {
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
				if err := validateTime(value); err != nil {
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
				if err := validateEmail(value); err != nil {
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

// parseTypedLiteral parses a typed literal (DATE, TIME, EMAIL)
// Format: TYPE 'value'
// Example: DATE '2024-01-13', TIME '14:30:00', EMAIL 'user@example.com'
func (p *Parser) parseTypedLiteral(kind ast.LiteralKind, validator func(string) error) (*ast.Literal, error) {
	typeKeyword := p.curTok.Literal
	p.nextToken() // consume type keyword (DATE/TIME/EMAIL)

	if p.curTok.Type != lexer.STRING {
		return nil, fmt.Errorf("expected string literal after %s, got %s", typeKeyword, p.curTok.Literal)
	}

	value := p.curTok.Literal
	
	// Validate the format
	if err := validator(value); err != nil {
		return nil, fmt.Errorf("%s validation failed: %w", typeKeyword, err)
	}

	p.nextToken()
	return &ast.Literal{
		TokenLiteralValue: typeKeyword + " '" + value + "'",
		Value:             value,
		Kind:              kind,
	}, nil
}

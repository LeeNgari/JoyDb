package parser

import (
	"fmt"
	"strings"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseCreate parses CREATE DATABASE or CREATE TABLE statement
func (p *Parser) parseCreate() (ast.Statement, error) {
	// Check what we are creating
	if p.peekTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.peekTok.Literal) == "TABLE" {
		stmt := p.parseCreateTable()
		if stmt == nil {
			return nil, fmt.Errorf("failed to parse CREATE TABLE statement")
		}
		
		// Optional semicolon
		if p.peekTok.Type == lexer.SEMICOLON {
			p.nextToken()
		}
		return stmt, nil
	}

	// Expect DATABASE token
	if !p.expectPeek(lexer.DATABASE) {
		return nil, fmt.Errorf("expected DATABASE or TABLE after CREATE, got %s", p.peekTok.Literal)
	}

	// Expect identifier (database name)
	if !p.expectPeek(lexer.IDENTIFIER) {
		return nil, fmt.Errorf("expected database name, got %s", p.peekTok.Literal)
	}

	stmt := &ast.CreateDatabaseStatement{
		Name: p.curTok.Literal,
	}

	// Optional semicolon
	if p.peekTok.Type == lexer.SEMICOLON {
		p.nextToken()
	}

	return stmt, nil
}

// parseDrop parses DROP DATABASE or DROP TABLE statement
func (p *Parser) parseDrop() (ast.Statement, error) {
	// Check what we are dropping
	if p.peekTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.peekTok.Literal) == "TABLE" {
		stmt := p.parseDropTable()
		if stmt == nil {
			return nil, fmt.Errorf("failed to parse DROP TABLE statement")
		}
		
		// Optional semicolon
		if p.peekTok.Type == lexer.SEMICOLON {
			p.nextToken()
		}
		return stmt, nil
	}

	// Expect DATABASE token
	if !p.expectPeek(lexer.DATABASE) {
		return nil, fmt.Errorf("expected DATABASE or TABLE after DROP, got %s", p.peekTok.Literal)
	}

	// Expect identifier (database name)
	if !p.expectPeek(lexer.IDENTIFIER) {
		return nil, fmt.Errorf("expected database name, got %s", p.peekTok.Literal)
	}

	stmt := &ast.DropDatabaseStatement{
		Name: p.curTok.Literal,
	}

	// Optional semicolon
	if p.peekTok.Type == lexer.SEMICOLON {
		p.nextToken()
	}

	return stmt, nil
}

// parseUse parses USE statement
func (p *Parser) parseUse() (ast.Statement, error) {
	// Expect identifier (database name)
	if !p.expectPeek(lexer.IDENTIFIER) {
		return nil, fmt.Errorf("expected database name after USE, got %s", p.peekTok.Literal)
	}

	stmt := &ast.UseDatabaseStatement{
		Name: p.curTok.Literal,
	}

	// Optional semicolon
	if p.peekTok.Type == lexer.SEMICOLON {
		p.nextToken()
	}

	return stmt, nil
}

// parseAlter parses ALTER DATABASE statement
func (p *Parser) parseAlter() (ast.Statement, error) {
	// Expect DATABASE token
	if !p.expectPeek(lexer.DATABASE) {
		return nil, fmt.Errorf("expected DATABASE after ALTER, got %s", p.peekTok.Literal)
	}

	// Expect identifier (database name)
	if !p.expectPeek(lexer.IDENTIFIER) {
		return nil, fmt.Errorf("expected database name, got %s", p.peekTok.Literal)
	}
	dbName := p.curTok.Literal

	// Expect RENAME token
	if !p.expectPeek(lexer.RENAME) {
		return nil, fmt.Errorf("expected RENAME after database name, got %s", p.peekTok.Literal)
	}

	// Expect TO token
	if !p.expectPeek(lexer.TO) {
		return nil, fmt.Errorf("expected TO after RENAME, got %s", p.peekTok.Literal)
	}

	// Expect identifier (new database name)
	if !p.expectPeek(lexer.IDENTIFIER) {
		return nil, fmt.Errorf("expected new database name, got %s", p.peekTok.Literal)
	}
	newDbName := p.curTok.Literal

	stmt := &ast.AlterDatabaseStatement{
		Name:    dbName,
		NewName: newDbName,
	}

	// Optional semicolon
	if p.peekTok.Type == lexer.SEMICOLON {
		p.nextToken()
	}

	return stmt, nil
}

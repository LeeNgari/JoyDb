package parser

import (
	"strings"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseCreateTable parses a CREATE TABLE statement
// Expected syntax: CREATE TABLE <name> ( <col_def>, ... )
// Where <col_def> is: <name> <type> [PRIMARY KEY] [AUTO_INCREMENT] [UNIQUE] [NOT NULL]
func (p *Parser) parseCreateTable() *ast.CreateTableStatement {
	stmt := &ast.CreateTableStatement{}

	// Skip CREATE, curTok becomes TABLE
	p.nextToken()

	// Skip TABLE, expect next to be identifier (table name)
	if !p.expectPeek(lexer.IDENTIFIER) {
		return nil
	}
	stmt.TableName = p.curTok.Literal

	// Ensure next is '('
	if !p.expectPeek(lexer.PAREN_OPEN) {
		return nil
	}
	p.nextToken() // Skip '(' so curTok is the first part of column def

	// Parse columns
	stmt.Columns = p.parseColumnDefs()
	if stmt.Columns == nil {
		return nil // Error parsing columns
	}

	return stmt
}

// parseColumnDefs parses the list of column definitions inside the parentheses
func (p *Parser) parseColumnDefs() []*ast.ColumnDef {
	var columns []*ast.ColumnDef

	// Parse first column
	if p.curTok.Type == lexer.PAREN_CLOSE {
		return nil
	}

	col := p.parseColumnDef()
	if col == nil {
		return nil
	}
	columns = append(columns, col)

	// Parse subsequent columns if separated by commas
	for p.curTok.Type == lexer.COMMA {
		p.nextToken() // Skip COMMA

		col := p.parseColumnDef()
		if col == nil {
			return nil
		}
		columns = append(columns, col)
	}

	// Ensure we end with ')'
	if p.curTok.Type != lexer.PAREN_CLOSE {
		return nil
	}

	return columns
}

// parseColumnDef parses a single column definition
// Syntax: <name> <type> [PRIMARY KEY] [AUTO_INCREMENT] [UNIQUE] [NOT NULL]
func (p *Parser) parseColumnDef() *ast.ColumnDef {
	if p.curTok.Type != lexer.IDENTIFIER && p.curTok.Type < lexer.SELECT {
		return nil
	}

	col := &ast.ColumnDef{
		Name: p.curTok.Literal,
	}

	p.nextToken() // Move to type

	// Ensure it's a valid data type (INT, TEXT, etc.)
	// The lexer parses these as identifier keywords.
	// Note: Some like DATE, TIME, EMAIL are parsed as specific tokens, not IDENTIFIER.
	// So we just accept any token that is an IDENTIFIER or a keyword.
	if p.curTok.Type != lexer.IDENTIFIER && p.curTok.Type < lexer.SELECT {
		return nil
	}
	col.Type = strings.ToUpper(p.curTok.Literal)

	// Check for optional modifiers (PRIMARY KEY, AUTO_INCREMENT, UNIQUE, NOT NULL)
	p.nextToken()
	
	// Modifiers can come in any order
	for {
		if p.curTok.Type == lexer.IDENTIFIER {
			upperLit := strings.ToUpper(p.curTok.Literal)
			if upperLit == "PRIMARY" {
				if p.peekTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.peekTok.Literal) == "KEY" {
					p.nextToken() // consume KEY
					col.PrimaryKey = true
				} else {
					return nil
				}
			} else if upperLit == "AUTO_INCREMENT" {
				col.AutoIncrement = true
			} else if upperLit == "UNIQUE" {
				col.Unique = true
			} else if upperLit == "NOT" {
				if p.peekTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.peekTok.Literal) == "NULL" {
					p.nextToken() // consume NULL
					col.NotNull = true
				} else {
					return nil
				}
			} else {
				// Not a modifier, must be a comma or rparen, break out
				break
			}
		} else {
			break
		}
		
		// Move to the next token, which might be another modifier, a comma, or rparen
		p.nextToken()
	}

	return col
}

// parseDropTable parses a DROP TABLE statement
// Expected syntax: DROP TABLE <name>
func (p *Parser) parseDropTable() *ast.DropTableStatement {
	stmt := &ast.DropTableStatement{}

	// Skip DROP, curTok becomes TABLE
	p.nextToken()

	// Skip TABLE, expect next to be identifier (table name)
	if !p.expectPeek(lexer.IDENTIFIER) {
		return nil
	}
	stmt.TableName = p.curTok.Literal

	return stmt
}

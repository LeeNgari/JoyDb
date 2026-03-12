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

	// Skip CREATE (already checked by Parse())
	p.nextToken()

	// Ensure next is TABLE
	if p.curTok.Type != lexer.IDENTIFIER || strings.ToUpper(p.curTok.Literal) != "TABLE" {
		return nil
	}
	p.nextToken() // consume TABLE

	// Table name
	if p.curTok.Type != lexer.IDENTIFIER {
		return nil
	}
	stmt.TableName = p.curTok.Literal
	p.nextToken()

	// Ensure '('
	if p.curTok.Type != lexer.PAREN_OPEN {
		return nil
	}
	p.nextToken() // consume '('

	// Parse columns
	stmt.Columns = p.parseColumnDefs()
	if stmt.Columns == nil {
		return nil
	}

	return stmt
}

// parseColumnDefs parses the list of column definitions inside the parentheses
func (p *Parser) parseColumnDefs() []*ast.ColumnDef {
	var columns []*ast.ColumnDef

	// Parse first column
	col := p.parseColumnDef()
	if col == nil {
		return nil
	}
	columns = append(columns, col)

	// Parse subsequent columns if separated by commas
	for p.curTok.Type == lexer.COMMA {
		p.nextToken() // consume COMMA

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
	p.nextToken() // consume ')'

	return columns
}

// parseColumnDef parses a single column definition
// Syntax: <name> <type> [PRIMARY KEY] [AUTO_INCREMENT] [UNIQUE] [NOT NULL]
func (p *Parser) parseColumnDef() *ast.ColumnDef {
	if !isIdentifierOrKeyword(p.curTok.Type) {
		return nil
	}

	col := &ast.ColumnDef{
		Name: p.curTok.Literal,
	}

	p.nextToken() // Move to type

	// Ensure it's a valid data type (INT, TEXT, etc.)
	// The lexer parses these as identifier keywords.
	// Note: Some like DATE, TIME, EMAIL are parsed as specific tokens, not IDENTIFIER.
	if !isIdentifierOrKeyword(p.curTok.Type) {
		return nil
	}
	col.Type = strings.ToUpper(p.curTok.Literal)
	p.nextToken() // Move past type to first modifier or comma/)

	// Check for optional modifiers (PRIMARY KEY, AUTO_INCREMENT, UNIQUE, NOT NULL)
	// Modifiers can come in any order
	for {
		if p.curTok.Type == lexer.IDENTIFIER {
			upperLit := strings.ToUpper(p.curTok.Literal)
			if upperLit == "PRIMARY" {
				if p.peekTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.peekTok.Literal) == "KEY" {
					p.nextToken() // consume PRIMARY (curTok -> KEY)
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
					p.nextToken() // consume NOT (curTok -> NULL)
					col.NotNull = true
				} else {
					return nil
				}
			} else if upperLit == "KEY" || upperLit == "NULL" {
				// Already handled as part of composite modifiers
			} else {
				// Not a modifier, break out
				break
			}
			p.nextToken() // Advance past modifier (or its last word)
		} else {
			break
		}
	}

	return col
}

// parseDropTable parses a DROP TABLE statement
// Expected syntax: DROP TABLE <name>
func (p *Parser) parseDropTable() *ast.DropTableStatement {
	stmt := &ast.DropTableStatement{}

	// Skip DROP (already checked by Parse())
	p.nextToken()

	// Ensure next is TABLE
	if p.curTok.Type != lexer.IDENTIFIER || strings.ToUpper(p.curTok.Literal) != "TABLE" {
		return nil
	}
	p.nextToken() // consume TABLE

	// Table name
	if p.curTok.Type != lexer.IDENTIFIER {
		return nil
	}
	stmt.TableName = p.curTok.Literal
	p.nextToken()

	return stmt
}

package parser

import (
	"strings"

	"github.com/leengari/joydb/internal/parser/ast"
	"github.com/leengari/joydb/internal/parser/lexer"
)

// parseCreateTable parses a CREATE TABLE statement
// Expected syntax: CREATE TABLE <name> ( <col_def>, ... )
// Where <col_def> is: <name> <type> [PRIMARY KEY] [AUTO_INCREMENT] [UNIQUE] [NOT NULL]
// Or table-level foreign key: FOREIGN KEY (col) REFERENCES ref_table(ref_col)
func (p *Parser) parseCreateTable() *ast.CreateTableStatement {
	stmt := &ast.CreateTableStatement{
		ForeignKeys: make([]*ast.ForeignKey, 0),
	}

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

	// Parse columns and foreign keys
	for {
		if p.curTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.curTok.Literal) == "FOREIGN" {
			// Parse table-level foreign key!
			p.nextToken() // consume FOREIGN
			if p.curTok.Type != lexer.IDENTIFIER || strings.ToUpper(p.curTok.Literal) != "KEY" {
				return nil
			}
			p.nextToken() // consume KEY

			if p.curTok.Type != lexer.PAREN_OPEN {
				return nil
			}
			p.nextToken() // consume '('

			if p.curTok.Type != lexer.IDENTIFIER {
				return nil
			}
			colName := p.curTok.Literal
			p.nextToken()

			if p.curTok.Type != lexer.PAREN_CLOSE {
				return nil
			}
			p.nextToken() // consume ')'

			if p.curTok.Type != lexer.IDENTIFIER || strings.ToUpper(p.curTok.Literal) != "REFERENCES" {
				return nil
			}
			p.nextToken() // consume REFERENCES

			if p.curTok.Type != lexer.IDENTIFIER {
				return nil
			}
			refTable := p.curTok.Literal
			p.nextToken()

			if p.curTok.Type != lexer.PAREN_OPEN {
				return nil
			}
			p.nextToken() // consume '('

			if p.curTok.Type != lexer.IDENTIFIER {
				return nil
			}
			refCol := p.curTok.Literal
			p.nextToken()

			if p.curTok.Type != lexer.PAREN_CLOSE {
				return nil
			}
			p.nextToken() // consume ')'

			stmt.ForeignKeys = append(stmt.ForeignKeys, &ast.ForeignKey{
				ColumnName:    colName,
				RefTableName:  refTable,
				RefColumnName: refCol,
			})
		} else {
			// Parse column definition
			col := p.parseColumnDef()
			if col == nil {
				return nil
			}

			// Support column-level inline foreign key: REFERENCES parent_table(parent_col)
			if p.curTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.curTok.Literal) == "REFERENCES" {
				p.nextToken() // consume REFERENCES
				if p.curTok.Type != lexer.IDENTIFIER {
					return nil
				}
				refTable := p.curTok.Literal
				p.nextToken()

				if p.curTok.Type != lexer.PAREN_OPEN {
					return nil
				}
				p.nextToken() // consume '('

				if p.curTok.Type != lexer.IDENTIFIER {
					return nil
				}
				refCol := p.curTok.Literal
				p.nextToken()

				if p.curTok.Type != lexer.PAREN_CLOSE {
					return nil
				}
				p.nextToken() // consume ')'

				stmt.ForeignKeys = append(stmt.ForeignKeys, &ast.ForeignKey{
					ColumnName:    col.Name,
					RefTableName:  refTable,
					RefColumnName: refCol,
				})
			}

			stmt.Columns = append(stmt.Columns, col)
		}

		if p.curTok.Type == lexer.COMMA {
			p.nextToken() // consume COMMA
			continue
		}
		if p.curTok.Type == lexer.PAREN_CLOSE {
			p.nextToken() // consume ')'
			break
		}
		return nil // illegal token
	}

	return stmt
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
	modifiersLoop:
	for {
		if p.curTok.Type == lexer.IDENTIFIER {
			upperLit := strings.ToUpper(p.curTok.Literal)
			switch upperLit {
			case "PRIMARY":
				if p.peekTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.peekTok.Literal) == "KEY" {
					p.nextToken() // consume PRIMARY (curTok -> KEY)
					col.PrimaryKey = true
				} else {
					return nil
				}
			case "AUTO_INCREMENT":
				col.AutoIncrement = true
			case "UNIQUE":
				col.Unique = true
			case "NOT":
				if p.peekTok.Type == lexer.NULL_KW {
					p.nextToken() // consume NOT (curTok -> NULL)
					col.NotNull = true
				} else {
					return nil
				}
			case "KEY":
			case "NULL":
				// Already handled as part of composite modifiers
			default:
				// Not a modifier, break out
				break modifiersLoop
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

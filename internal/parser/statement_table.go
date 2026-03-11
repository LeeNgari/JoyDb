package parser

import (
	"fmt"
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

	fmt.Printf("After CREATE, curTok: %v, peekTok: %v\n", p.curTok, p.peekTok)

	// Skip TABLE, expect next to be identifier (table name)
	if !p.expectPeek(lexer.IDENTIFIER) {
		fmt.Printf("Expected IDENTIFIER for table name, got %v\n", p.peekTok)
		return nil
	}
	stmt.TableName = p.curTok.Literal
	fmt.Printf("TableName: %s\n", stmt.TableName)

	// Ensure next is '('
	if !p.expectPeek(lexer.PAREN_OPEN) {
		fmt.Printf("Expected PAREN_OPEN, got %v\n", p.peekTok)
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
	fmt.Printf("Inside parseColumnDefs, curTok: %v, peekTok: %v\n", p.curTok, p.peekTok)
	var columns []*ast.ColumnDef

	// Parse first column
	if p.curTok.Type == lexer.PAREN_CLOSE {
		fmt.Println("Error: Immediate PAREN_CLOSE")
		return nil
	}

	col := p.parseColumnDef()
	if col == nil {
		fmt.Println("Error: First column parsed as nil")
		return nil
	}
	columns = append(columns, col)

	// Parse subsequent columns if separated by commas
	for p.peekTok.Type == lexer.COMMA {
		p.nextToken() // Skip current token (which was the last token of previous col)
		p.nextToken() // Skip COMMA

		col := p.parseColumnDef()
		if col == nil {
			return nil
		}
		columns = append(columns, col)
	}

	// Ensure we end with ')'
	if !p.expectPeek(lexer.PAREN_CLOSE) {
		return nil
	}

	return columns
}

// parseColumnDef parses a single column definition
// Syntax: <name> <type> [PRIMARY KEY] [AUTO_INCREMENT] [UNIQUE] [NOT NULL]
func (p *Parser) parseColumnDef() *ast.ColumnDef {
	fmt.Printf("Inside parseColumnDef, curTok: %v, peekTok: %v\n", p.curTok, p.peekTok)
	if p.curTok.Type != lexer.IDENTIFIER {
		fmt.Printf("Expected IDENTIFIER for col name, got %v\n", p.curTok)
		return nil
	}

	col := &ast.ColumnDef{
		Name: p.curTok.Literal,
	}

	p.nextToken() // Move to type
	fmt.Printf("After moving to type, curTok: %v\n", p.curTok)

	// Ensure it's a valid data type (INT, TEXT, etc.)
	// The lexer parses these as identifier keywords.
	// Note: Some like DATE, TIME, EMAIL are parsed as specific tokens, not IDENTIFIER.
	// So we just accept any token that is an IDENTIFIER or a keyword.
	if p.curTok.Type != lexer.IDENTIFIER && p.curTok.Type < lexer.SELECT {
		fmt.Printf("Expected IDENTIFIER or keyword for col type, got %v\n", p.curTok)
		return nil
	}
	col.Type = strings.ToUpper(p.curTok.Literal)

	// Check for optional modifiers (PRIMARY KEY, AUTO_INCREMENT, UNIQUE, NOT NULL)
	p.nextToken()
	
	// Modifiers can come in any order
	for {
		fmt.Printf("Modifier loop, curTok: %v\n", p.curTok)
		if p.curTok.Type == lexer.IDENTIFIER {
			upperLit := strings.ToUpper(p.curTok.Literal)
			if upperLit == "PRIMARY" {
				if p.peekTok.Type == lexer.IDENTIFIER && strings.ToUpper(p.peekTok.Literal) == "KEY" {
					p.nextToken() // consume KEY
					col.PrimaryKey = true
				} else {
					fmt.Printf("Expected KEY after PRIMARY, got %v\n", p.peekTok)
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
					fmt.Printf("Expected NULL after NOT, got %v\n", p.peekTok)
					return nil
				}
			} else {
				// Not a modifier, must be a comma or rparen, break out
				fmt.Printf("Breaking modifier loop on IDENTIFIER %s\n", upperLit)
				break
			}
		} else {
			fmt.Printf("Breaking modifier loop on non-IDENTIFIER %v\n", p.curTok.Type)
			break
		}
		
		// Move to the next token, which might be another modifier, a comma, or rparen
		p.nextToken()
	}

	// We don't advance the token at the very end because the caller's loop
	// handles checking for COMMA or RPAREN on curTok.
	fmt.Printf("Returning col from parseColumnDef: %v\n", col)
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

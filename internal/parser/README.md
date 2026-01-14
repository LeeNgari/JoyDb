# Parser Package

The parser package converts a stream of tokens from the lexer into an Abstract Syntax Tree (AST) that represents the structure of SQL statements.

## Architecture

The parser uses a **recursive descent** parsing approach with **operator precedence climbing** for expressions.

### File Organization

| File | Responsibility | LOC |
|------|---------------|-----|
| `parser.go` | Main parser struct and entry point | ~50 |
| `statement_select.go` | SELECT and JOIN parsing | ~140 |
| `statement_insert.go` | INSERT parsing | ~70 |
| `statement_update.go` | UPDATE parsing | ~100 |
| `statement_delete.go` | DELETE parsing | ~50 |
| `expression.go` | Expression parsing (AND/OR/comparisons) | ~100 |
| `identifier.go` | Identifier and column name parsing | ~130 |
| `literal.go` | Literal value parsing | ~160 |
| `helpers.go` | Utility functions | ~40 |
| `validators.go` | Type validation wrappers | ~20 |

## Usage

```go
import (
    "github.com/leengari/mini-rdbms/internal/parser"
    "github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// Tokenize SQL
tokens, err := lexer.Tokenize("SELECT * FROM users WHERE id = 5")
if err != nil {
    // Handle error
}

// Parse tokens into AST
p := parser.New(tokens)
stmt, err := p.Parse()
if err != nil {
    // Handle error
}

// stmt is now an ast.Statement (e.g., *ast.SelectStatement)
```

## Supported Statements

- **SELECT**: `SELECT fields FROM table [JOIN ...] [WHERE condition]`
- **INSERT**: `INSERT INTO table (columns) VALUES (values)`
- **UPDATE**: `UPDATE table SET col=val [WHERE condition]`
- **DELETE**: `DELETE FROM table [WHERE condition]`

## Expression Parsing

The parser handles complex expressions with proper operator precedence:

1. **Comparison operators** (=, <, >, <=, >=, !=, <>) - Highest precedence
2. **AND** - Higher than OR
3. **OR** - Lowest precedence

Parentheses can override precedence:
```sql
WHERE (age > 18 OR premium = true) AND active = true
```

## Adding a New Statement Type

To add support for a new SQL statement (e.g., `TRUNCATE`):

### 1. Add Token Types (if needed)

Edit `lexer/lexer.go`:
```go
const (
    // ... existing tokens
    TRUNCATE
)

var keywords = map[string]TokenType{
    // ... existing keywords
    "TRUNCATE": TRUNCATE,
}
```

### 2. Define AST Node

Edit `ast/nodes.go`:
```go
// TruncateStatement: TRUNCATE TABLE table_name
type TruncateStatement struct {
    TableName *Identifier
}

func (s *TruncateStatement) statementNode()       {}
func (s *TruncateStatement) TokenLiteral() string { return "TRUNCATE" }
func (s *TruncateStatement) String() string {
    return "TRUNCATE TABLE " + s.TableName.String()
}
```

### 3. Create Statement Parser

Create `statement_truncate.go`:
```go
package parser

import (
    "fmt"
    "github.com/leengari/mini-rdbms/internal/parser/ast"
    "github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// parseTruncate parses a TRUNCATE statement
// Grammar: TRUNCATE TABLE table_name
func (p *Parser) parseTruncate() (*ast.TruncateStatement, error) {
    stmt := &ast.TruncateStatement{}
    
    // TRUNCATE keyword - already consumed by Parse()
    p.nextToken()
    
    // TABLE keyword (optional in some SQL dialects)
    if p.curTok.Type == lexer.TABLE {
        p.nextToken()
    }
    
    // Table name
    if p.curTok.Type != lexer.IDENTIFIER {
        return nil, fmt.Errorf("expected table name, got %s", p.curTok.Literal)
    }
    stmt.TableName = &ast.Identifier{
        TokenLiteralValue: p.curTok.Literal,
        Value: p.curTok.Literal,
    }
    p.nextToken()
    
    // Semicolon (optional)
    if p.curTok.Type == lexer.SEMICOLON {
        p.nextToken()
    }
    
    return stmt, nil
}
```

### 4. Update Parser Entry Point

Edit `parser.go`:
```go
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
    case lexer.TRUNCATE:  // Add this case
        return p.parseTruncate()
    default:
        return nil, fmt.Errorf("unexpected token %v", p.curTok.Type)
    }
}
```

### 5. Create Executor

Create `executor/truncate_executor.go` (see executor README for details)

### 6. Add Tests

Create tests in `parser_test.go`:
```go
func TestParseTruncate(t *testing.T) {
    input := "TRUNCATE TABLE users"
    tokens, _ := lexer.Tokenize(input)
    p := New(tokens)
    stmt, err := p.Parse()
    
    if err != nil {
        t.Fatalf("Parse error: %v", err)
    }
    
    truncStmt, ok := stmt.(*ast.TruncateStatement)
    if !ok {
        t.Fatalf("Expected *ast.TruncateStatement, got %T", stmt)
    }
    
    if truncStmt.TableName.Value != "users" {
        t.Errorf("Expected table 'users', got '%s'", truncStmt.TableName.Value)
    }
}
```

## Design Principles

1. **Single Responsibility**: Each file handles one type of parsing
2. **No Side Effects**: Parsing only creates AST nodes, no execution
3. **Error Recovery**: Clear error messages with token position
4. **Lookahead**: Uses `peekTok` for one-token lookahead
5. **Immutability**: AST nodes are immutable once created

## Common Patterns

### Parsing Lists

```go
// Parse comma-separated items
for {
    item, err := p.parseItem()
    if err != nil {
        return nil, err
    }
    items = append(items, item)
    
    if p.curTok.Type != lexer.COMMA {
        break
    }
    p.nextToken() // consume comma
}
```

### Optional Clauses

```go
// Parse optional WHERE clause
if p.curTok.Type == lexer.WHERE {
    p.nextToken()
    expr, err := p.parseExpression()
    if err != nil {
        return nil, err
    }
    stmt.Where = expr
}
```

### Lookahead for Disambiguation

```go
// Check if keyword is used as identifier or literal
if p.curTok.Type == lexer.DATE {
    p.nextToken()
    if p.curTok.Type == lexer.STRING {
        // It's a typed literal: DATE '2024-01-14'
        return p.parseTypedLiteral(...)
    } else {
        // It's a column name
        return &ast.Identifier{Value: "date"}
    }
}
```

## Testing

Run parser tests:
```bash
go test ./internal/parser/...
```

Run specific test:
```bash
go test ./internal/parser -run TestParseSelect
```

## Related Packages

- `lexer/` - Tokenization
- `ast/` - AST node definitions
- `executor/` - AST execution

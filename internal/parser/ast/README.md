# AST (Abstract Syntax Tree) Package

The AST package defines the node types that represent the structure of parsed SQL statements. Each node type implements the `Node` interface and represents a specific SQL construct.

## Node Hierarchy

```
Node (interface)
├── Statement (interface)
│   ├── SelectStatement
│   ├── InsertStatement
│   ├── UpdateStatement
│   └── DeleteStatement
└── Expression (interface)
    ├── Identifier
    ├── Literal
    ├── BinaryExpression
    └── LogicalExpression
```

## Node Types

### Statements

| Type | Represents | Example |
|------|-----------|---------|
| `SelectStatement` | SELECT queries | `SELECT * FROM users WHERE id = 5` |
| `InsertStatement` | INSERT operations | `INSERT INTO users (id, name) VALUES (1, 'Alice')` |
| `UpdateStatement` | UPDATE operations | `UPDATE users SET name = 'Bob' WHERE id = 1` |
| `DeleteStatement` | DELETE operations | `DELETE FROM users WHERE id = 1` |

### Expressions

| Type | Represents | Example |
|------|-----------|---------|
| `Identifier` | Column/table names | `users.id`, `name` |
| `Literal` | Constant values | `5`, `'hello'`, `true`, `DATE '2024-01-14'` |
| `BinaryExpression` | Comparisons | `id = 5`, `age > 18` |
| `LogicalExpression` | Logical operations | `age > 18 AND active = true` |

### Supporting Types

| Type | Purpose |
|------|---------|
| `JoinClause` | Represents JOIN operations |
| `LiteralKind` | Enum for literal types (INT, FLOAT, STRING, BOOL, DATE, TIME, EMAIL) |

## Node Interfaces

All nodes implement the `Node` interface:

```go
type Node interface {
    TokenLiteral() string  // Returns the token that started this node
    String() string        // Returns a string representation for debugging
}
```

**Statements** additionally implement:
```go
type Statement interface {
    Node
    statementNode()  // Marker method
}
```

**Expressions** additionally implement:
```go
type Expression interface {
    Node
    expressionNode()  // Marker method
}
```

## Usage Example

```go
import "github.com/leengari/mini-rdbms/internal/parser/ast"

// Create a SELECT statement AST manually
stmt := &ast.SelectStatement{
    Fields: []*ast.Identifier{
        {Value: "id"},
        {Value: "name"},
    },
    TableName: &ast.Identifier{Value: "users"},
    Where: &ast.BinaryExpression{
        Left: &ast.Identifier{Value: "id"},
        Operator: "=",
        Right: &ast.Literal{Value: 5, Kind: ast.LiteralInt},
    },
}

// Print the statement
fmt.Println(stmt.String())
// Output: SELECT id, name FROM users WHERE (id = 5)
```

## Adding a New Node Type

### 1. Define the Struct

Add to `nodes.go`:

```go
// TruncateStatement: TRUNCATE TABLE table_name
// Represents a TRUNCATE statement that removes all rows from a table
type TruncateStatement struct {
    TableName *Identifier
}
```

### 2. Implement Node Interface

```go
func (s *TruncateStatement) statementNode()       {}
func (s *TruncateStatement) TokenLiteral() string { return "TRUNCATE" }
func (s *TruncateStatement) String() string {
    return "TRUNCATE TABLE " + s.TableName.String()
}
```

### 3. Add to Parser

Create `parser/statement_truncate.go` to parse into this node type (see parser README).

### 4. Add to Executor

Create `executor/truncate_executor.go` to execute this node type (see executor README).

## Node Design Principles

1. **Immutability**: Nodes should not be modified after creation
2. **Self-Describing**: `String()` method provides readable representation
3. **Type Safety**: Use specific types rather than `interface{}`
4. **Minimal Logic**: Nodes are data structures, not behavior

## Literal Types

The `Literal` node supports multiple data types via the `LiteralKind` enum:

```go
const (
    LiteralString LiteralKind = "STRING"  // 'hello'
    LiteralInt    LiteralKind = "INT"     // 42
    LiteralFloat  LiteralKind = "FLOAT"   // 3.14
    LiteralBool   LiteralKind = "BOOL"    // true, false
    LiteralDate   LiteralKind = "DATE"    // DATE '2024-01-14'
    LiteralTime   LiteralKind = "TIME"    // TIME '14:30:00'
    LiteralEmail  LiteralKind = "EMAIL"   // EMAIL 'user@example.com'
)
```

### Adding a New Literal Type

1. Add to `LiteralKind` enum:
```go
const (
    // ... existing types
    LiteralUUID LiteralKind = "UUID"
)
```

2. Add lexer token (if needed):
```go
// In lexer/lexer.go
const (
    // ... existing tokens
    UUID
)
```

3. Add parser support:
```go
// In parser/literal.go
case lexer.UUID:
    value := p.curTok.Literal
    if err := validateUUID(value); err != nil {
        return nil, err
    }
    return &ast.Literal{
        Value: value,
        Kind: ast.LiteralUUID,
    }, nil
```

## Qualified Identifiers

Identifiers can be qualified with a table name:

```go
// Unqualified
&ast.Identifier{
    Value: "id",
    Table: "",
}

// Qualified
&ast.Identifier{
    Value: "id",
    Table: "users",
}
```

The `String()` method handles both:
```go
func (i *Identifier) String() string {
    if i.Table != "" {
        return i.Table + "." + i.Value
    }
    return i.Value
}
```

## JOIN Clauses

JOIN operations are represented by `JoinClause`:

```go
type JoinClause struct {
    JoinType    string      // "INNER", "LEFT", "RIGHT", "FULL"
    RightTable  *Identifier // Table to join with
    OnCondition Expression  // JOIN condition (e.g., users.id = orders.user_id)
}
```

Example:
```go
join := &ast.JoinClause{
    JoinType: "INNER",
    RightTable: &ast.Identifier{Value: "orders"},
    OnCondition: &ast.BinaryExpression{
        Left: &ast.Identifier{Table: "users", Value: "id"},
        Operator: "=",
        Right: &ast.Identifier{Table: "orders", Value: "user_id"},
    },
}
```

## Testing AST Nodes

Test node creation and string representation:

```go
func TestSelectStatement(t *testing.T) {
    stmt := &ast.SelectStatement{
        Fields: []*ast.Identifier{{Value: "*"}},
        TableName: &ast.Identifier{Value: "users"},
    }
    
    expected := "SELECT * FROM users"
    if stmt.String() != expected {
        t.Errorf("Expected %q, got %q", expected, stmt.String())
    }
}
```

## Related Packages

- `parser/` - Creates AST from tokens
- `executor/` - Executes AST nodes
- `lexer/` - Provides tokens for parser

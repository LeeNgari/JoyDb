# Parser Layer

## What

The Parser Layer converts **SQL text into an Abstract Syntax Tree (AST)** - a structured representation of the query that can be analyzed and executed. It consists of three components:

1. **Lexer**: Breaks SQL text into tokens (keywords, identifiers, operators, literals)
2. **Parser**: Builds an AST from tokens using recursive descent parsing
3. **AST**: Defines node types representing SQL constructs

## Why

### Design Rationale

**Why separate lexing and parsing?**
- **Modularity**: Lexer handles character-level concerns, parser handles syntax structure
- **Reusability**: Tokens can be used for syntax highlighting, formatting, etc.
- **Simplicity**: Each component has a single, focused responsibility

**Why build an AST instead of executing directly?**
- **Validation**: Can validate syntax without executing
- **Optimization**: Planner can analyze and optimize the AST
- **Flexibility**: Same AST can be executed in different ways
- **Testing**: Can test parsing independently of execution

**Why recursive descent parsing?**
- **Simplicity**: Easy to understand and maintain
- **Flexibility**: Easy to add new statement types
- **Error messages**: Can provide clear error messages with context
- **No dependencies**: No external parser generator needed

## How

### Three-Stage Process

```
SQL Text → Lexer → Tokens → Parser → AST → (to Planner)
```

#### Stage 1: Lexical Analysis (Lexer)

**Input**: `"SELECT * FROM users WHERE id = 5"`

**Output**: Token stream
```
[SELECT, ASTERISK, FROM, IDENTIFIER("users"), WHERE, IDENTIFIER("id"), EQUALS, NUMBER(5)]
```

**Mechanism**:
- Reads SQL character by character
- Recognizes keywords (case-insensitive)
- Identifies operators (`=`, `<`, `>`, `<=`, `>=`, `!=`, `<>`)
- Extracts literals (strings in `'quotes'`, numbers, booleans)
- Tracks line and column numbers for error reporting

**Key Features**:
- Case-insensitive keywords: `SELECT` = `select` = `SeLeCt`
- Multi-character operators: `<=`, `>=`, `!=`, `<>`
- String literals in single quotes: `'hello world'`
- Numeric literals: integers (`42`) and floats (`3.14`)
- Boolean literals: `true`, `false`

---

#### Stage 2: Syntax Analysis (Parser)

**Input**: Token stream from lexer

**Output**: Abstract Syntax Tree (AST)

**Mechanism**:
- Uses **recursive descent** parsing
- One function per grammar rule
- **Operator precedence climbing** for expressions
- **One-token lookahead** for disambiguation

**Parsing Strategy**:
```
Parse() → parseSelect() / parseInsert() / parseUpdate() / parseDelete()
                ↓
        parseExpression() → parseBinaryExpression()
                ↓                      ↓
        parseIdentifier()      parseLiteral()
```

**Operator Precedence** (highest to lowest):
1. Comparison operators (`=`, `<`, `>`, `<=`, `>=`, `!=`, `<>`)
2. `AND`
3. `OR`

Parentheses `()` override precedence.

---

#### Stage 3: AST Representation

**Input**: Parser output

**Output**: Typed AST nodes

**Node Hierarchy**:
```
Node (interface)
├── Statement
│   ├── SelectStatement
│   ├── InsertStatement
│   ├── UpdateStatement
│   ├── DeleteStatement
│   ├── CreateDatabaseStatement
│   ├── UseDatabaseStatement
│   └── DropDatabaseStatement
└── Expression
    ├── Identifier (column/table names)
    ├── Literal (values)
    ├── BinaryExpression (comparisons)
    └── LogicalExpression (AND/OR)
```

**Example AST** for `SELECT * FROM users WHERE id = 5`:
```go
&SelectStatement{
    Fields: []*Identifier{{Value: "*"}},
    TableName: &Identifier{Value: "users"},
    Where: &BinaryExpression{
        Left: &Identifier{Value: "id"},
        Operator: "=",
        Right: &Literal{Value: 5, Kind: LiteralInt},
    },
}
```

## Interactions

### With Engine Layer
- Engine calls `lexer.Tokenize(sql)` to get tokens
- Engine calls `parser.New(tokens).Parse()` to get AST
- Engine passes AST to Planner

### With Planner Layer
- Parser produces AST nodes
- Planner validates AST and converts to execution plan
- Planner doesn't modify AST (immutable)

## Supported Statements

### Data Query Language (DQL)
- **SELECT**: `SELECT fields FROM table [JOIN ...] [WHERE condition]`

### Data Manipulation Language (DML)
- **INSERT**: `INSERT INTO table (columns) VALUES (values)`
- **UPDATE**: `UPDATE table SET col=val [WHERE condition]`
- **DELETE**: `DELETE FROM table [WHERE condition]`

### Database Management
- **CREATE DATABASE**: `CREATE DATABASE name`
- **USE**: `USE database_name`
- **DROP DATABASE**: `DROP DATABASE name`
- **ALTER DATABASE**: `ALTER DATABASE old_name RENAME TO new_name`

### JOIN Operations
- **INNER JOIN**: Returns only matching rows
- **LEFT JOIN**: Returns all left rows + matches
- **RIGHT JOIN**: Returns all right rows + matches
- **FULL OUTER JOIN**: Returns all rows from both tables

## Expression Parsing

### Comparison Operators
`=`, `!=`, `<>`, `<`, `>`, `<=`, `>=`

### Logical Operators
`AND`, `OR`

### Precedence Example
```sql
WHERE age > 18 AND active = true OR premium = true
```
Parsed as:
```
OR
├── AND
│   ├── age > 18
│   └── active = true
└── premium = true
```

With parentheses:
```sql
WHERE age > 18 AND (active = true OR premium = true)
```
Parsed as:
```
AND
├── age > 18
└── OR
    ├── active = true
    └── premium = true
```

## Literal Types

| Type | Example | AST Kind |
|------|---------|----------|
| Integer | `42`, `-10` | `LiteralInt` |
| Float | `3.14`, `-0.5` | `LiteralFloat` |
| String | `'hello'`, `'user@example.com'` | `LiteralString` |
| Boolean | `true`, `false` | `LiteralBool` |
| Date | `DATE '2024-01-14'` | `LiteralDate` |
| Time | `TIME '14:30:00'` | `LiteralTime` |
| Email | `EMAIL 'user@example.com'` | `LiteralEmail` |

## Design Principles

1. **Single Responsibility**: Lexer tokenizes, parser builds AST, AST represents structure
2. **No Side Effects**: Parsing only creates AST nodes, no execution or validation
3. **Immutability**: AST nodes are immutable once created
4. **Error Recovery**: Clear error messages with token position
5. **Type Safety**: Specific node types for each SQL construct

## Error Handling

### Lexical Errors
```sql
SELECT * FROM users WHERE name = 'unterminated
```
Error: `illegal token at line 1, col 35: unterminated string`

### Syntax Errors
```sql
SELECT * users WHERE id = 5
```
Error: `parse error: expected FROM, got IDENTIFIER`

### Position Tracking
All errors include line and column numbers for easy debugging.

## Key Components

### Lexer (`lexer/lexer.go`)

**Main Functions**:
- `Tokenize(input string) ([]Token, error)` - Convenience function
- `New(input string) *Lexer` - Create lexer
- `NextToken() Token` - Get next token

**Token Types**:
- Keywords: `SELECT`, `FROM`, `WHERE`, `INSERT`, `UPDATE`, `DELETE`, `JOIN`, etc.
- Operators: `ASTERISK`, `COMMA`, `LPAREN`, `RPAREN`, `EQUALS`, `LESS_THAN`, etc.
- Literals: `IDENTIFIER`, `STRING`, `NUMBER`
- Special: `EOF`, `ILLEGAL`

---

### Parser (`parser.go`)

**Main Functions**:
- `New(tokens []Token) *Parser` - Create parser
- `Parse() (Statement, error)` - Parse tokens into AST

**Statement Parsers** (one file per statement type):
- `parseSelect()` - `statement_select.go`
- `parseInsert()` - `statement_insert.go`
- `parseUpdate()` - `statement_update.go`
- `parseDelete()` - `statement_delete.go`

**Expression Parsers**:
- `parseExpression()` - `expression.go`
- `parseIdentifier()` - `identifier.go`
- `parseLiteral()` - `literal.go`

---

### AST (`ast/nodes.go`)

**Node Interfaces**:
```go
type Node interface {
    TokenLiteral() string  // Token that started this node
    String() string        // String representation
}

type Statement interface {
    Node
    statementNode()  // Marker method
}

type Expression interface {
    Node
    expressionNode()  // Marker method
}
```

**Statement Types**:
- `SelectStatement`, `InsertStatement`, `UpdateStatement`, `DeleteStatement`
- `CreateDatabaseStatement`, `UseDatabaseStatement`, `DropDatabaseStatement`

**Expression Types**:
- `Identifier`, `Literal`, `BinaryExpression`, `LogicalExpression`


## Related Documentation

- [Planner Layer](../planner/README.md) - Converts AST to execution plans
- [Executor Layer](../executor/README.md) - Executes parsed statements
- [ARCHITECTURE.md](../../ARCHITECTURE.md) - System architecture overview


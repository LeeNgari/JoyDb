# Planning Layer

## What

The Planning Layer converts **Abstract Syntax Trees (AST) into executable query plans**. It acts as the bridge between syntax analysis (Parser) and execution (Executor), performing validation, type conversion, and predicate building.

**Key Components**:
- **Planner** (`planner/planner.go`): Converts AST statements to Plan nodes
- **Plan Nodes** (`plan/nodes.go`): Typed execution instructions
- **Predicate Builder** (`planner/predicate/`): Converts AST expressions to predicate functions

## Why

### Design Rationale

**Why separate planning from parsing?**
- **Validation**: Parser validates syntax, Planner validates semantics (table/column existence)
- **Type safety**: Planner converts AST literals to correct types based on schema
- **Optimization**: Planner can optimize queries without re-parsing
- **Clarity**: Separates "what the user wrote" (AST) from "what to execute" (Plan)

**Why separate planning from execution?**
- **Testability**: Can test planning logic without executing queries
- **Caching**: Could cache execution plans for repeated queries (future enhancement)
- **Flexibility**: Same plan can be executed in different ways (e.g., explain plan)

**Why use predicate functions?**
- **Performance**: Compiled Go functions are faster than interpreting AST at runtime
- **Simplicity**: Executors work with simple `func(Row) bool` instead of complex AST traversal
- **Type safety**: Type conversion happens once during planning, not per row

## How

### Planning Pipeline

```
AST Statement → Planner → Plan Node → (to Executor)
      ↓                        ↓
  Validation            Predicate Functions
  Type Conversion       Projection Specs
  Table/Column Lookup   JOIN Configurations
```

### Process Steps

#### 1. Statement Dispatch

```go
func Plan(stmt ast.Statement, db *schema.Database) (plan.Node, error) {
    switch s := stmt.(type) {
    case *ast.SelectStatement:
        return planSelect(s, db)
    case *ast.InsertStatement:
        return planInsert(s, db)
    case *ast.UpdateStatement:
        return planUpdate(s, db)
    case *ast.DeleteStatement:
        return planDelete(s, db)
    }
}
```

#### 2. Validation

**Table Existence**:
```go
table, ok := db.Tables[tableName]
if !ok {
    return nil, fmt.Errorf("table not found: %s", tableName)
}
```

**Column Existence** (for INSERT/UPDATE):
```go
schemaCol := findColumnInSchema(table, colName)
if schemaCol == nil {
    return nil, fmt.Errorf("column not found: %s", colName)
}
```

#### 3. Type Conversion

**Convert AST literals to schema types**:
```go
// AST has: Literal{Value: "2024-01-14", Kind: LiteralString}
// Schema expects: ColumnTypeDate

convertedLit, err := types.ConvertLiteralToSchemaType(lit, schemaCol.Type)
// Result: Literal{Value: "2024-01-14", Kind: LiteralDate}
```

#### 4. Predicate Building

**Convert AST WHERE clause to predicate function**:

AST:
```go
&BinaryExpression{
    Left: &Identifier{Value: "age"},
    Operator: ">",
    Right: &Literal{Value: 18, Kind: LiteralInt},
}
```

Plan:
```go
predicate := func(row data.Row) bool {
    age, ok := row["age"].(int)
    if !ok {
        return false
    }
    return age > 18
}
```

#### 5. Projection Building

**Convert field list to projection spec**:

AST:
```go
Fields: []*Identifier{
    {Value: "id"},
    {Value: "username"},
}
```

Plan:
```go
&projection.Projection{
    SelectAll: false,
    Columns: []projection.ColumnRef{
        {Column: "id"},
        {Column: "username"},
    },
}
```

#### 6. JOIN Configuration

**Convert JOIN clauses to JOIN nodes**:

AST:
```go
&JoinClause{
    JoinType: "INNER",
    RightTable: &Identifier{Value: "orders"},
    OnCondition: &BinaryExpression{
        Left: &Identifier{Value: "id", Table: "users"},
        Operator: "=",
        Right: &Identifier{Value: "user_id", Table: "orders"},
    },
}
```

Plan:
```go
plan.JoinNode{
    TargetTable: "orders",
    JoinType: join.JoinTypeInner,
    LeftOnCol: "id",
    RightOnCol: "user_id",
}
```

## Plan Node Types

### SelectNode
```go
type SelectNode struct {
    TableName  string
    Predicate  func(data.Row) bool  // WHERE clause
    Projection *projection.Projection
    Joins      []JoinNode
}
```

### InsertNode
```go
type InsertNode struct {
    TableName string
    Row       data.Row  // Pre-converted values
}
```

### UpdateNode
```go
type UpdateNode struct {
    TableName string
    Predicate func(data.Row) bool  // WHERE clause
    Updates   data.Row  // Columns to update
}
```

### DeleteNode
```go
type DeleteNode struct {
    TableName string
    Predicate func(data.Row) bool  // WHERE clause
}
```

## Interactions

### With Parser Layer
- Receives AST from Parser
- Does NOT modify AST (immutable)
- Validates AST semantics

### With Domain Layer
- Accesses Database to validate tables/columns
- Uses schema information for type conversion
- Doesn't modify database (read-only)

### With Executor Layer
- Produces Plan nodes for Executor
- Executor dispatches based on Plan node type
- Executor doesn't need to know about AST

## Predicate Builder

**Location**: `planner/predicate/builder.go`

**Purpose**: Convert AST expressions to predicate functions

### Supported Expressions

#### Binary Expressions (Comparisons)
```go
// age > 18
predicate.Build(&BinaryExpression{
    Left: &Identifier{Value: "age"},
    Operator: ">",
    Right: &Literal{Value: 18},
})
// Returns: func(row) bool { return row["age"] > 18 }
```

#### Logical Expressions (AND/OR)
```go
// age > 18 AND active = true
predicate.Build(&LogicalExpression{
    Left: &BinaryExpression{...},  // age > 18
    Operator: "AND",
    Right: &BinaryExpression{...}, // active = true
})
// Returns: func(row) bool { return (age > 18) && (active == true) }
```

#### Qualified Identifiers
```go
// users.id = 5
predicate.Build(&BinaryExpression{
    Left: &Identifier{Table: "users", Value: "id"},
    Operator: "=",
    Right: &Literal{Value: 5},
})
// Returns: func(row) bool { return row["users.id"] == 5 }
```

### Value Comparison

Uses `util/types.CompareValues()` for type-safe comparisons:
```go
// Handles: int, float, string, bool
// Operators: =, !=, <>, <, >, <=, >=
CompareValues(42, ">", 18)  // true
CompareValues("alice", "=", "bob")  // false
```

## Design Decisions

### Why Build Predicates Instead of Interpreting AST?
**Trade-off**: Planning time vs. execution time
- **Current**: Convert AST to predicate once during planning
- **Alternative**: Interpret AST for each row during execution
- **Reason**: Execution happens more frequently than planning, so optimize for execution

### Why Pre-Convert Values in INSERT/UPDATE?
**Trade-off**: Planning complexity vs. execution simplicity
- **Current**: Convert all values during planning, fail fast on type errors
- **Alternative**: Convert during execution
- **Reason**: Fail fast - catch type errors before modifying data

### Why Validate Tables/Columns During Planning?
**Trade-off**: Planning overhead vs. execution safety
- **Current**: Validate everything during planning
- **Alternative**: Validate during execution
- **Reason**: Fail fast - don't start execution if query is invalid

## Limitations

### Current Limitations
1. **No query optimization**: Executes queries as written
2. **No predicate pushdown**: Filters applied after JOINs
3. **No index selection**: Doesn't choose optimal indexes
4. **No cost estimation**: Doesn't estimate query cost
5. **No plan caching**: Re-plans identical queries

### Future Enhancements
- **Query optimization**: Predicate pushdown, join reordering
- **Index selection**: Choose best index for WHERE clauses
- **Cost-based optimization**: Estimate and minimize query cost
- **Plan caching**: Cache plans for repeated queries
- **Prepared statements**: Pre-plan queries with parameters

## Example: SELECT Planning

### Input AST
```sql
SELECT username, email FROM users WHERE age > 18
```

### Planning Steps

1. **Validate table**: Check `users` exists in database
2. **Build predicate**: Convert `age > 18` to `func(row) bool { return row["age"] > 18 }`
3. **Build projection**: Create projection for `[username, email]`
4. **Create plan node**:
```go
&SelectNode{
    TableName: "users",
    Predicate: func(row data.Row) bool {
        age, ok := row["age"].(int)
        return ok && age > 18
    },
    Projection: &projection.Projection{
        SelectAll: false,
        Columns: []projection.ColumnRef{
            {Column: "username"},
            {Column: "email"},
        },
    },
    Joins: nil,
}
```

### Output
Plan node ready for execution by Executor.



## Related Documentation

- [Parser Layer](../parser/README.md) - Produces AST for planning
- [Executor Layer](../executor/README.md) - Executes plan nodes
- [Domain Layer](../domain/README.md) - Database schema and tables
- [ARCHITECTURE.md](../../ARCHITECTURE.md) - System architecture overview

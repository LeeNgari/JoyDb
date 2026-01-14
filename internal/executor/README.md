# Executor Package

The executor package is responsible for executing parsed SQL statements against the database. It acts as a bridge between the parser (AST) and the query engine.

## Architecture

```
AST Statement → Executor → Query Operations → Database
```

The executor:
1. Receives AST statements from the parser
2. Validates table/column existence
3. Builds predicates from WHERE clauses
4. Calls appropriate query operations
5. Returns formatted results

## File Organization

| File | Responsibility | LOC |
|------|---------------|-----|
| `executor.go` | Main entry point, Execute() dispatcher | ~50 |
| `select_executor.go` | SELECT statement execution | ~95 |
| `insert_executor.go` | INSERT statement execution | ~60 |
| `update_executor.go` | UPDATE statement execution | ~72 |
| `delete_executor.go` | DELETE statement execution | ~46 |
| `join_executor.go` | JOIN SELECT execution | ~192 |
| `predicate/builder.go` | Predicate building from AST | ~105 |
| `predicate/README.md` | Predicate package documentation | - |

## Usage

```go
import (
    "github.com/leengari/mini-rdbms/internal/executor"
    "github.com/leengari/mini-rdbms/internal/parser"
)

// Parse SQL
stmt, err := parser.Parse(tokens)

// Execute statement
result, err := executor.Execute(stmt, database)

// Access results
for _, row := range result.Rows {
    fmt.Println(row)
}
```

## Statement Execution Flow

### SELECT
```
AST SelectStatement
  ↓
select_executor.go
  ↓
Build Projection + Predicate
  ↓
crud.SelectWhere() or join.ExecuteJoin()
  ↓
Result with Rows
```

### INSERT
```
AST InsertStatement
  ↓
insert_executor.go
  ↓
Type Conversion (util/types)
  ↓
crud.Insert()
  ↓
Result with Message
```

### UPDATE
```
AST UpdateStatement
  ↓
update_executor.go
  ↓
Build Predicate + Type Conversion
  ↓
crud.Update()
  ↓
Result with Rows Affected
```

### DELETE
```
AST DeleteStatement
  ↓
delete_executor.go
  ↓
Build Predicate
  ↓
crud.Delete()
  ↓
Result with Rows Affected
```

## Design Principles

1. **Single Responsibility**: Each executor handles one statement type
2. **Separation of Concerns**: Executors don't implement query logic, they delegate
3. **Type Safety**: Uses util/types for type conversion and comparison
4. **Error Handling**: Clear error messages with context

## Predicate Building

The `predicate` subpackage handles converting AST expressions into executable predicates:

```go
// Build predicate from WHERE clause
pred, err := predicate.Build(whereExpression)

// Use predicate to filter rows
matchingRows := crud.SelectWhere(table, pred, projection)
```

See `predicate/README.md` for details.

## Adding a New Statement Executor

To add support for a new statement type (e.g., `TRUNCATE`):

### 1. Create Executor File

Create `truncate_executor.go`:

```go
package executor

import (
    "fmt"
    "github.com/leengari/mini-rdbms/internal/domain/schema"
    "github.com/leengari/mini-rdbms/internal/parser/ast"
)

// executeTruncate handles TRUNCATE statements
func executeTruncate(stmt *ast.TruncateStatement, db *schema.Database) (*Result, error) {
    tableName := stmt.TableName.Value
    table, ok := db.Tables[tableName]
    if !ok {
        return nil, fmt.Errorf("table not found: %s", tableName)
    }
    
    // Clear all rows (implementation depends on storage layer)
    table.Rows = []data.Row{}
    
    return &Result{
        Message: fmt.Sprintf("TRUNCATE TABLE %s", tableName),
    }, nil
}
```

### 2. Update Main Dispatcher

Edit `executor.go`:

```go
func Execute(stmt ast.Statement, db *schema.Database) (*Result, error) {
    switch s := stmt.(type) {
    case *ast.SelectStatement:
        return executeSelect(s, db)
    case *ast.InsertStatement:
        return executeInsert(s, db)
    case *ast.UpdateStatement:
        return executeUpdate(s, db)
    case *ast.DeleteStatement:
        return executeDelete(s, db)
    case *ast.TruncateStatement:  // Add this
        return executeTruncate(s, db)
    default:
        return nil, fmt.Errorf("unsupported statement type: %T", stmt)
    }
}
```

### 3. Add Tests

Create tests in `executor_test.go` or integration tests.

## Result Structure

```go
type Result struct {
    Columns      []string         // Column names for SELECT
    Metadata     []ColumnMetadata // Column type information
    Rows         []data.Row       // Result rows for SELECT
    Message      string           // Status message
    RowsAffected int              // For INSERT/UPDATE/DELETE
}
```

## Type Conversion

Executors use `util/types` for type conversion:

```go
// Convert string literal to DATE if schema expects DATE
convertedLit, err := types.ConvertLiteralToSchemaType(lit, schemaCol.Type)
```

This enables implicit type detection based on schema.

## Error Handling

Executors provide clear, contextual error messages:

```go
// Table not found
return nil, fmt.Errorf("table not found: %s", tableName)

// Type mismatch
return nil, fmt.Errorf("column '%s': %w", colName, err)

// Predicate building failed
return nil, fmt.Errorf("failed to build WHERE predicate: %w", err)
```

## Testing

Run executor tests:
```bash
# Integration tests (recommended)
go test ./internal/integration_test/...

# Build verification
go build ./...
```

## Related Packages

- `parser/` - Provides AST statements
- `query/operations/crud/` - CRUD operations
- `query/operations/join/` - JOIN operations
- `query/operations/projection/` - Column projection
- `util/types/` - Type conversion and comparison
- `predicate/` - Predicate building

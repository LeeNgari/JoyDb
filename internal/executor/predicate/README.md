# Predicate Package

The predicate package provides utilities for building predicate functions from AST expressions. Predicates are used to filter rows in WHERE clauses.

## Responsibility

Convert AST expressions into executable predicate functions that test whether a row matches certain criteria.

## Usage

```go
import (
    "github.com/leengari/mini-rdbms/internal/executor/predicate"
    "github.com/leengari/mini-rdbms/internal/parser/ast"
)

// Build predicate from WHERE clause expression
pred, err := predicate.Build(whereExpression)
if err != nil {
    // Handle error
}

// Use predicate to filter rows
for _, row := range table.Rows {
    if pred(row) {
        // Row matches the condition
    }
}
```

## Supported Expressions

### Comparison Operators
- `=` - Equal to
- `!=`, `<>` - Not equal to
- `<` - Less than
- `>` - Greater than
- `<=` - Less than or equal
- `>=` - Greater than or equal

### Logical Operators
- `AND` - Both conditions must be true
- `OR` - Either condition must be true

### Examples

```sql
-- Simple comparison
WHERE age > 18

-- Logical AND
WHERE age >= 18 AND active = true

-- Logical OR
WHERE status = 'pending' OR status = 'processing'

-- Nested expressions
WHERE (age > 18 OR premium = true) AND active = true
```

## Architecture

```
AST Expression → predicate.Build() → PredicateFunc → Filter Rows
```

### Predicate Function Type

```go
type PredicateFunc func(data.Row) bool
```

A predicate function takes a row and returns true if it matches the condition.

## Implementation Details

### Recursive Descent

The builder uses recursive descent to handle nested expressions:

1. **Comparison expressions** → Direct predicate
2. **Logical expressions** → Recursively build left and right, then combine

### Column Name Resolution

Handles both qualified and unqualified column names:

```go
// Qualified: orders.amount
if tableName != "" {
    val = row[tableName + "." + colName]
}

// Unqualified: amount
if !ok {
    val = row[colName]
}
```

### Value Comparison

Uses `util/types.CompareValues()` for type-safe comparisons:
- Numeric comparison (int, float)
- String comparison (lexicographic)
- Boolean comparison (equality only)

## Design Principles

1. **Separation of Concerns**: Predicate building isolated from execution
2. **Reusability**: Used by both SELECT and JOIN executors
3. **Type Safety**: Leverages util/types for comparisons
4. **Error Handling**: Clear error messages for invalid expressions

## Related Packages

- `executor/` - Uses predicates for WHERE clause filtering
- `query/operations/crud/` - Defines PredicateFunc type
- `query/operations/join/` - Uses predicates for JOIN filtering
- `util/types/` - Provides value comparison logic

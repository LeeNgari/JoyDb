# Utilities Package

The `util` package provides shared utility functions used across multiple packages in the RDBMS project. This helps avoid code duplication and circular dependencies.

## Subpackages

### `util/types`

Type conversion, comparison, and validation utilities.

**Files:**
- `conversion.go` - Type conversion between AST literals and schema types
- `comparison.go` - Value comparison logic for WHERE clauses

**Key Functions:**

```go
// Convert AST literal to match schema type
func ConvertLiteralToSchemaType(lit *ast.Literal, schemaType schema.ColumnType) (*ast.Literal, error)

// Check if literal type matches schema type
func TypesMatch(kind ast.LiteralKind, schemaType schema.ColumnType) bool

// Validate literal type against expected type
func ValidateLiteralType(lit *ast.Literal, expectedType schema.ColumnType) error

// Compare two values with an operator (=, <, >, <=, >=, !=, <>)
func CompareValues(left interface{}, op string, right interface{}) bool

// Convert numeric types to float64
func NormalizeToFloat(v interface{}) (float64, bool)

// Convert numeric types to int64
func NormalizeToInt64(val interface{}) (int64, bool)
```

## Usage Examples

### Type Conversion

```go
import "github.com/leengari/mini-rdbms/internal/util/types"

// Convert string literal to DATE if schema expects DATE
lit := &ast.Literal{Value: "2024-01-14", Kind: ast.LiteralString}
converted, err := types.ConvertLiteralToSchemaType(lit, schema.ColumnTypeDate)
// converted.Kind == ast.LiteralDate
```

### Value Comparison

```go
import "github.com/leengari/mini-rdbms/internal/util/types"

// Compare values in WHERE clause
result := types.CompareValues(42, ">", 18)  // true
result = types.CompareValues("alice", "=", "bob")  // false
result = types.CompareValues(3.14, "<=", 5.0)  // true
```

### Type Normalization

```go
// Normalize different numeric types for comparison
val1, ok1 := types.NormalizeToFloat(42)      // 42.0, true
val2, ok2 := types.NormalizeToFloat(int64(5)) // 5.0, true
val3, ok3 := types.NormalizeToFloat("text")  // 0.0, false
```

## Design Principles

1. **No Dependencies on Executor**: Utilities should not depend on executor package
2. **Pure Functions**: No side effects, deterministic outputs
3. **Type Safety**: Clear type signatures and error handling
4. **Reusability**: Functions used by multiple packages (executor, query operations)

## Why This Package Exists

Before creating `util/`, type conversion and comparison logic was duplicated across:
- `executor/type_conversion.go`
- `executor/type_validation.go`
- `executor/executor.go` (compareValues, normalizeToFloat)
- `query/operations/crud/insert.go` (normalizeToInt64)

This caused:
- Code duplication
- Inconsistent behavior
- Difficulty in testing
- Potential circular dependencies when refactoring

By centralizing these utilities:
- Single source of truth
- Easier to test
- Reusable across all packages
- Avoids circular dependencies

## Adding New Utilities

### When to Add Here

Add utilities to this package when:
1. Logic is used by 2+ packages
2. Logic is pure (no side effects)
3. Logic doesn't belong to domain models
4. You want to avoid circular dependencies

### When NOT to Add Here

Don't add here if:
1. Logic is specific to one package
2. Logic has side effects (I/O, state changes)
3. Logic belongs in domain models (e.g., Table methods)

### Example: Adding UUID Support

1. Add to `conversion.go`:
```go
func ConvertToUUID(value string) (string, error) {
    // Validate UUID format
    if !isValidUUID(value) {
        return "", fmt.Errorf("invalid UUID format")
    }
    return value, nil
}
```

2. Add to `comparison.go` if needed:
```go
func CompareUUIDs(left, right string) bool {
    return strings.EqualFold(left, right)
}
```

## Testing

Test utilities in isolation:

```go
func TestCompareValues(t *testing.T) {
    tests := []struct{
        left, right interface{}
        op string
        want bool
    }{
        {42, 18, ">", true},
        {3.14, 3.14, "=", true},
        {"alice", "bob", "<", true},
    }
    
    for _, tt := range tests {
        got := types.CompareValues(tt.left, tt.op, tt.right)
        if got != tt.want {
            t.Errorf("CompareValues(%v, %s, %v) = %v, want %v",
                tt.left, tt.op, tt.right, got, tt.want)
        }
    }
}
```

## Related Packages

- `executor/` - Uses these utilities for type handling
- `query/operations/crud/` - Uses for INSERT/UPDATE operations
- `domain/schema/` - Defines ColumnType enum
- `parser/ast/` - Defines LiteralKind enum

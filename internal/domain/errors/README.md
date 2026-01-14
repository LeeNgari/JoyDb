# Domain Errors Package

This package defines all custom error types used throughout the RDBMS project. Using custom error types provides:
- Better error context and debugging information
- Type-safe error handling with `errors.As()`
- Consistent error messages
- Easier error testing

## Error Types

### Constraint Errors (`constraint.go`)

**ConstraintError** - Database constraint violations

```go
// Unique constraint violation
err := errors.NewUniqueViolation("users", "email", "test@example.com", []int{5, 10})

// Not null violation
err := errors.NewNotNullViolation("users", "username", 3)

// Primary key violation
err := errors.NewPrimaryKeyViolation("users", "id", 42)

// Type mismatch
err := errors.NewTypeMismatch("users", "age", "abc", "INT")
```

### Parse Errors (`parse.go`)

**ParseError** - SQL parsing errors with position tracking

```go
// Simple parse error
err := errors.NewParseError("unexpected token", "FROM")

// With position
err := errors.NewParseErrorWithPosition("expected SELECT", "SELCT", 1, 5)

// Wrapping another error
err := errors.NewParseErrorWithCause("validation failed", validationErr)
```

### Execution Errors (`execution.go`)

**ExecutionError** - SQL execution errors

```go
// Execution error
err := errors.NewExecutionError("INSERT", "users", "duplicate key")

// Wrapping another error
err := errors.NewExecutionErrorWithCause("UPDATE", "users", constraintErr)
```

**TableNotFoundError** - Table doesn't exist

```go
err := errors.NewTableNotFoundError("nonexistent_table")
```

**ColumnNotFoundError** - Column doesn't exist

```go
err := errors.NewColumnNotFoundError("users", "invalid_column")
```

### Validation Errors (`validation.go`)

**ValidationError** - Data validation errors

```go
err := errors.NewValidationError("users", "email", "invalid", "EMAIL", "invalid format")
```

**StorageError** - File system and storage errors

```go
// Storage error
err := errors.NewStorageError("load", "/path/to/db", "file not found")

// Wrapping another error
err := errors.NewStorageErrorWithCause("save", "/path/to/db", ioErr)
```

## Usage Patterns

### Creating Errors

```go
// Use specific error constructors
return errors.NewTableNotFoundError(tableName)

// Wrap existing errors
if err != nil {
    return errors.NewStorageErrorWithCause("load", dbPath, err)
}
```

### Checking Error Types

```go
var tableErr *errors.TableNotFoundError
if errors.As(err, &tableErr) {
    fmt.Printf("Table %s not found\n", tableErr.TableName)
}

var constraintErr *errors.ConstraintError
if errors.As(err, &constraintErr) {
    fmt.Printf("Constraint violation: %s\n", constraintErr.Constraint)
}
```

### Error Wrapping

All custom errors that wrap other errors implement `Unwrap()`:

```go
// Check wrapped error
if errors.Is(err, io.EOF) {
    // Handle EOF
}

// Unwrap manually
cause := errors.Unwrap(err)
```

## Design Principles

1. **Specific Types**: Each error type represents a specific failure mode
2. **Rich Context**: Errors include relevant details (table, column, value, etc.)
3. **Error Wrapping**: Support for wrapping underlying errors with `%w`
4. **Consistent Format**: Predictable error message structure
5. **Type Safety**: Use `errors.As()` for type-safe error handling

## Migration from fmt.Errorf

**Before**:
```go
return fmt.Errorf("table not found: %s", tableName)
```

**After**:
```go
return errors.NewTableNotFoundError(tableName)
```

**Before**:
```go
return fmt.Errorf("failed to load table: %w", err)
```

**After**:
```go
return errors.NewStorageErrorWithCause("load", tablePath, err)
```

## Testing

```go
func TestTableNotFound(t *testing.T) {
    err := doSomething()
    
    var tableErr *errors.TableNotFoundError
    if !errors.As(err, &tableErr) {
        t.Fatal("expected TableNotFoundError")
    }
    
    if tableErr.TableName != "users" {
        t.Errorf("expected table 'users', got '%s'", tableErr.TableName)
    }
}
```

## Related Packages

- `executor/` - Uses ExecutionError, TableNotFoundError
- `parser/` - Uses ParseError
- `storage/` - Uses StorageError
- `query/operations/crud/` - Uses ConstraintError
- `validation/` - Uses ValidationError

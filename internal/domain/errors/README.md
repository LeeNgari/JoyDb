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

package errors

import "fmt"

// ValidationError represents a data validation error
type ValidationError struct {
	Table    string
	Column   string
	Value    interface{}
	Expected string // Expected type or format
	Message  string
	RowIndex int // -1 if unknown
}

func (e *ValidationError) Error() string {
	if e.RowIndex >= 0 {
		return fmt.Sprintf("validation error in %s.%s at row %d: %s (got: %v, expected: %s)",
			e.Table, e.Column, e.RowIndex, e.Message, e.Value, e.Expected)
	}
	return fmt.Sprintf("validation error in %s.%s: %s (got: %v, expected: %s)",
		e.Table, e.Column, e.Message, e.Value, e.Expected)
}

// NewValidationError creates a new validation error
func NewValidationError(table, column string, value interface{}, expected, message string) *ValidationError {
	return &ValidationError{
		Table:    table,
		Column:   column,
		Value:    value,
		Expected: expected,
		Message:  message,
		RowIndex: -1,
	}
}

// StorageError represents a storage layer error
type StorageError struct {
	Operation string // "load", "save", "read", "write"
	Path      string // File or directory path
	Message   string
	Cause     error
}

func (e *StorageError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("storage error during %s of '%s': %s",
			e.Operation, e.Path, e.Message)
	}
	return fmt.Sprintf("storage error during %s: %s", e.Operation, e.Message)
}

func (e *StorageError) Unwrap() error {
	return e.Cause
}

// NewStorageError creates a new storage error
func NewStorageError(operation, path, message string) *StorageError {
	return &StorageError{
		Operation: operation,
		Path:      path,
		Message:   message,
	}
}

// NewStorageErrorWithCause creates a storage error wrapping another error
func NewStorageErrorWithCause(operation, path string, cause error) *StorageError {
	return &StorageError{
		Operation: operation,
		Path:      path,
		Message:   cause.Error(),
		Cause:     cause,
	}
}

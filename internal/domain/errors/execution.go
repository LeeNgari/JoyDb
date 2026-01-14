package errors

import "fmt"

// ExecutionError represents an error during SQL statement execution
type ExecutionError struct {
	Statement string // Type of statement (SELECT, INSERT, UPDATE, DELETE)
	Table     string // Table name (if applicable)
	Message   string // Error message
	Cause     error  // Underlying error
}

func (e *ExecutionError) Error() string {
	if e.Table != "" {
		return fmt.Sprintf("%s execution error on table '%s': %s", 
			e.Statement, e.Table, e.Message)
	}
	return fmt.Sprintf("%s execution error: %s", e.Statement, e.Message)
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// NewExecutionError creates a new execution error
func NewExecutionError(statement, table, message string) *ExecutionError {
	return &ExecutionError{
		Statement: statement,
		Table:     table,
		Message:   message,
	}
}

// NewExecutionErrorWithCause creates an execution error wrapping another error
func NewExecutionErrorWithCause(statement, table string, cause error) *ExecutionError {
	return &ExecutionError{
		Statement: statement,
		Table:     table,
		Message:   cause.Error(),
		Cause:     cause,
	}
}

// TableNotFoundError represents a table not found error
type TableNotFoundError struct {
	TableName string
}

func (e *TableNotFoundError) Error() string {
	return fmt.Sprintf("table not found: %s", e.TableName)
}

// NewTableNotFoundError creates a new table not found error
func NewTableNotFoundError(tableName string) *TableNotFoundError {
	return &TableNotFoundError{TableName: tableName}
}

// ColumnNotFoundError represents a column not found error
type ColumnNotFoundError struct {
	TableName  string
	ColumnName string
}

func (e *ColumnNotFoundError) Error() string {
	if e.TableName != "" {
		return fmt.Sprintf("column not found: %s.%s", e.TableName, e.ColumnName)
	}
	return fmt.Sprintf("column not found: %s", e.ColumnName)
}

// NewColumnNotFoundError creates a new column not found error
func NewColumnNotFoundError(tableName, columnName string) *ColumnNotFoundError {
	return &ColumnNotFoundError{
		TableName:  tableName,
		ColumnName: columnName,
	}
}

package errors

import (
	"fmt"
	"strings"
)

// ConstraintError represents a violation of a database constraint
// (unique, primary key, not null, type mismatch, foreign key later, etc.)
type ConstraintError struct {
	Table      string      // table name
	Column     string      // column name (empty if table-level constraint)
	Value      interface{} // offending value (may be nil)
	Constraint string      // "unique", "primary_key", "not_null", "type_mismatch", etc.
	Reason     string      // human-readable explanation (optional)
	RowIndex   int         // row number (0-based) where violation occurred (-1 if unknown)
	Rows       []int       // for unique violations: all conflicting row positions
}

func (e *ConstraintError) Error() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("constraint violation in %s.%s", e.Table, e.Column))

	if e.Constraint != "" {
		parts = append(parts, fmt.Sprintf("(%s)", e.Constraint))
	}

	if e.Value != nil {
		parts = append(parts, fmt.Sprintf("value=%v", e.Value))
	}

	if e.Reason != "" {
		parts = append(parts, e.Reason)
	}

	if e.RowIndex >= 0 {
		parts = append(parts, fmt.Sprintf("at row %d", e.RowIndex))
	}

	return fmt.Sprintf("%s", strings.Join(parts, " - "))
}

// NewUniqueViolation creates a unique constraint violation error
func NewUniqueViolation(table, column string, value interface{}, rows []int) *ConstraintError {
	return &ConstraintError{
		Table:      table,
		Column:     column,
		Value:      value,
		Constraint: "unique",
		Reason:     "duplicate value",
		RowIndex:   -1,
		Rows:       rows,
	}
}

// NewNotNullViolation creates a not null constraint violation error
func NewNotNullViolation(table, column string, rowIndex int) *ConstraintError {
	return &ConstraintError{
		Table:      table,
		Column:     column,
		Value:      nil,
		Constraint: "not_null",
		Reason:     "missing required value",
		RowIndex:   rowIndex,
	}
}

// NewPrimaryKeyViolation creates a primary key constraint violation error
func NewPrimaryKeyViolation(table, column string, value interface{}) *ConstraintError {
	return &ConstraintError{
		Table:      table,
		Column:     column,
		Value:      value,
		Constraint: "primary_key",
		Reason:     "duplicate primary key",
	}
}

// NewTypeMismatch creates a type mismatch constraint violation error
func NewTypeMismatch(table, column string, value interface{}, expectedType string) *ConstraintError {
	return &ConstraintError{
		Table:      table,
		Column:     column,
		Value:      value,
		Constraint: "type_mismatch",
		Reason:     fmt.Sprintf("expected type %s", expectedType),
	}
}

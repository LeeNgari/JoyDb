package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/errors"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
)

// Email validation regex - reasonable balance between strictness and practicality
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// ValidateRow checks if the given row matches the table's schema
// - Checks required fields (NOT NULL)
// - Validates type compatibility (with JSON reality)
// - Returns ConstraintError for better error handling
func ValidateRow(table *schema.Table, row data.Row, rowIndex int) error {
	for _, col := range table.Schema.Columns {
		val, exists := row[col.Name]

		// Handle missing value
		if !exists {
			if col.NotNull {
				return &errors.ConstraintError{
					Table:      table.Name,
					Column:     col.Name,
					Constraint: "not_null",
					Reason:     "missing required value",
					RowIndex:   rowIndex,
				}
			}
			continue // nullable
		}

		// Type validation
		switch col.Type {
		case schema.ColumnTypeText:
			if _, ok := val.(string); !ok {
				return &errors.ConstraintError{
					Table:      table.Name,
					Column:     col.Name,
					Value:      val,
					Constraint: "type_mismatch",
					Reason:     fmt.Sprintf("expected string, got %T", val),
					RowIndex:   rowIndex,
				}
			}
		case schema.ColumnTypeInt:
			// Normalize to int64 for consistency
			switch v := val.(type) {
			case int:
				row[col.Name] = int64(v) // normalize to int64
			case int64:
				// already int64, perfect
			case float64:
				// JSON numbers come as float64
				if v == float64(int64(v)) {
					row[col.Name] = int64(v)
				} else {
					return &errors.ConstraintError{
						Table:      table.Name,
						Column:     col.Name,
						Value:      val,
						Constraint: "type_mismatch",
						Reason:     fmt.Sprintf("expected integer, got float with decimal: %v", v),
						RowIndex:   rowIndex,
					}
				}
			default:
				return &errors.ConstraintError{
					Table:      table.Name,
					Column:     col.Name,
					Value:      val,
					Constraint: "type_mismatch",
					Reason:     fmt.Sprintf("expected integer, got %T", val),
					RowIndex:   rowIndex,
				}
			}
		case schema.ColumnTypeFloat:
			if _, ok := val.(float64); !ok {
				return typeMismatchError(table.Name, col.Name, val, "Float (number)", rowIndex)
			}

		case schema.ColumnTypeEmail:
			str, ok := val.(string)
			if !ok {
				return typeMismatchError(table.Name, col.Name, val, "string", rowIndex)
			}
			// Validate email format
			if !emailRegex.MatchString(str) {
				return &errors.ConstraintError{
					Table:      table.Name,
					Column:     col.Name,
					Value:      val,
					Constraint: "invalid_email",
					Reason:     "invalid email format",
					RowIndex:   rowIndex,
				}
			}

		case schema.ColumnTypeBool:
			if _, ok := val.(bool); !ok {
				return typeMismatchError(table.Name, col.Name, val, "boolean", rowIndex)
			}

		case schema.ColumnTypeDate, schema.ColumnTypeTime:
			switch v := val.(type) {
			case string:
				_, err := time.Parse("2006-01-02", v)
				if err != nil && col.Type == schema.ColumnTypeTime {
					_, err = time.Parse(time.RFC3339, v)
				}
				if err != nil {
					return typeMismatchError(table.Name, col.Name, val, "date/time string", rowIndex)
				}
			default:
				return typeMismatchError(table.Name, col.Name, val, "date/time string or time.Time", rowIndex)
			}

		default:
			return fmt.Errorf("unknown column type %q", col.Type)
		}
	}

	return nil
}

// typeMismatchError creates a type mismatch error
func typeMismatchError(table, col string, val interface{}, expected string, rowIndex int) *errors.ConstraintError {
	return &errors.ConstraintError{
		Table:      table,
		Column:     col,
		Value:      val,
		Constraint: "type_mismatch",
		Reason:     fmt.Sprintf("expected %s, got %T", expected, val),
		RowIndex:   rowIndex,
	}
}

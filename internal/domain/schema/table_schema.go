package schema

import (
	"github.com/leengari/mini-rdbms/internal/domain/errors"
)

// TableSchema represents table metadata (from meta.json)
type TableSchema struct {
	TableName string
	Columns   []Column
}

// Validate checks if the table schema is valid (e.g., has at least one column)
func (s *TableSchema) Validate() error {
	if len(s.Columns) == 0 {
		return &errors.ConstraintError{
			Table:      s.TableName,
			Constraint: "schema",
			Reason:     "table must have at least one column",
		}
	}
	return nil
}

// GetPrimaryKeyColumn returns the primary key column if it exists
func (s *TableSchema) GetPrimaryKeyColumn() *Column {
	for i := range s.Columns {
		if s.Columns[i].PrimaryKey {
			return &s.Columns[i]
		}
	}
	return nil
}

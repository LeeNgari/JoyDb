package schema

import "fmt"

// TableSchema represents table metadata (from meta.json)
type TableSchema struct {
	TableName string
	Columns   []Column
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

// Validate ensures the schema is valid
func (s *TableSchema) Validate() error {
	if len(s.Columns) == 0 {
		return fmt.Errorf("table must have at least one column")
	}
	return nil
}

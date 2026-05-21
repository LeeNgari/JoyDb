package schema

import "fmt"

// ForeignKey represents a foreign key constraint on the table
type ForeignKey struct {
	ColumnName    string `json:"column_name"`
	RefTableName  string `json:"ref_table_name"`
	RefColumnName string `json:"ref_column_name"`
}

// TableSchema represents table metadata (from meta.json)
type TableSchema struct {
	TableName   string       `json:"table_name"`
	Columns     []Column     `json:"columns"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
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

func (s *TableSchema) Validate() error {
	if len(s.Columns) == 0 {
		return fmt.Errorf("table must have at least one column")
	}
	if s.GetPrimaryKeyColumn() == nil {
		return fmt.Errorf("table must define a PRIMARY KEY column")
	}
	return nil
}

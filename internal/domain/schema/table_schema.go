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

// GetColumnIndex returns the index of the column with the given name, or -1 if not found.
func (s *TableSchema) GetColumnIndex(name string) int {
	for i := range s.Columns {
		if s.Columns[i].Name == name {
			return i
		}
	}
	return -1
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

// ColumnNames returns a slice of column names in schema order.
func (s *TableSchema) ColumnNames() []string {
	names := make([]string, len(s.Columns))
	for i := range s.Columns {
		names[i] = s.Columns[i].Name
	}
	return names
}

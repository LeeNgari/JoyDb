package metadata

// DatabaseMeta represents the database-level metadata from meta.json
type DatabaseMeta struct {
	Name    string   `json:"name"`
	Version int      `json:"version"`
	Tables  []string `json:"tables,omitempty"`
}

// TableMeta represents the table-level metadata from meta.json
type TableMeta struct {
	Name         string       `json:"name"`
	Columns      []ColumnMeta `json:"columns"`
	LastInsertID int64        `json:"last_insert_id,omitempty"`
	RowCount     int64        `json:"row_count,omitempty"`
}

// ColumnMeta represents column metadata for JSON serialization
type ColumnMeta struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	PrimaryKey    bool   `json:"primary_key"`
	Unique        bool   `json:"unique"`
	NotNull       bool   `json:"not_null"`
	AutoIncrement bool   `json:"auto_increment,omitempty"`
}

package schema

type ColumnType string

const (
	ColumnTypeInt   ColumnType = "INT"
	ColumnTypeFloat ColumnType = "FLOAT"
	ColumnTypeText  ColumnType = "TEXT"
	ColumnTypeBool  ColumnType = "BOOL"
	ColumnTypeDate  ColumnType = "DATE"
	ColumnTypeTime  ColumnType = "TIME"
	ColumnTypeEmail ColumnType = "EMAIL"
)

type Column struct {
	Name          string     `json:"name"`
	Type          ColumnType `json:"type"`
	PrimaryKey    bool       `json:"primary_key"`
	Unique        bool       `json:"unique"`
	NotNull       bool       `json:"not_null"`
	AutoIncrement bool       `json:"auto_increment,omitempty"`
}

package schema

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestTableSchema_Validate(t *testing.T) {
	t.Run("ValidSchema", func(t *testing.T) {
		schema := &TableSchema{
			TableName: "users",
			Columns: []Column{
				{Name: "id", Type: ColumnTypeInt, PrimaryKey: true},
			},
		}
		err := schema.Validate()
		assert.NoError(t, err)
	})

	t.Run("InvalidSchema_NoColumns", func(t *testing.T) {
		schema := &TableSchema{
			TableName: "users",
			Columns:   []Column{},
		}
		err := schema.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "table must have at least one column")
	})
}

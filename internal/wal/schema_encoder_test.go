package wal

import (
	"testing"
	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/stretchr/testify/assert"
)

func TestSchemaEncoderRoundTrip(t *testing.T) {
	s := &schema.TableSchema{
		TableName: "users",
		Columns: []schema.Column{
			{Name: "id", Type: "INT", PrimaryKey: true, AutoIncrement: true, NotNull: true},
			{Name: "email", Type: "EMAIL", Unique: true, NotNull: true},
			{Name: "bio", Type: "TEXT"},
		},
		ForeignKeys: []schema.ForeignKey{
			{ColumnName: "id", RefTableName: "parent", RefColumnName: "parent_id"},
		},
	}

	// Encode
	data := EncodeTableSchema(s)
	assert.NotEmpty(t, data)

	// Decode
	decoded, err := DecodeTableSchema(data)
	assert.NoError(t, err)

	assert.Equal(t, s.TableName, decoded.TableName)
	assert.Equal(t, len(s.Columns), len(decoded.Columns))
	assert.Equal(t, len(s.ForeignKeys), len(decoded.ForeignKeys))

	for i := range s.Columns {
		assert.Equal(t, s.Columns[i].Name, decoded.Columns[i].Name)
		assert.Equal(t, s.Columns[i].Type, decoded.Columns[i].Type)
		assert.Equal(t, s.Columns[i].PrimaryKey, decoded.Columns[i].PrimaryKey)
		assert.Equal(t, s.Columns[i].AutoIncrement, decoded.Columns[i].AutoIncrement)
		assert.Equal(t, s.Columns[i].Unique, decoded.Columns[i].Unique)
		assert.Equal(t, s.Columns[i].NotNull, decoded.Columns[i].NotNull)
	}

	for i := range s.ForeignKeys {
		assert.Equal(t, s.ForeignKeys[i].ColumnName, decoded.ForeignKeys[i].ColumnName)
		assert.Equal(t, s.ForeignKeys[i].RefTableName, decoded.ForeignKeys[i].RefTableName)
		assert.Equal(t, s.ForeignKeys[i].RefColumnName, decoded.ForeignKeys[i].RefColumnName)
	}
}

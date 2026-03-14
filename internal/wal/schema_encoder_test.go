package wal

import (
	"testing"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
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
	}

	// Encode
	data := EncodeTableSchema(s)
	assert.NotEmpty(t, data)

	// Decode
	decoded, err := DecodeTableSchema(data)
	assert.NoError(t, err)

	assert.Equal(t, s.TableName, decoded.TableName)
	assert.Equal(t, len(s.Columns), len(decoded.Columns))

	for i := range s.Columns {
		assert.Equal(t, s.Columns[i].Name, decoded.Columns[i].Name)
		assert.Equal(t, s.Columns[i].Type, decoded.Columns[i].Type)
		assert.Equal(t, s.Columns[i].PrimaryKey, decoded.Columns[i].PrimaryKey)
		assert.Equal(t, s.Columns[i].AutoIncrement, decoded.Columns[i].AutoIncrement)
		assert.Equal(t, s.Columns[i].Unique, decoded.Columns[i].Unique)
		assert.Equal(t, s.Columns[i].NotNull, decoded.Columns[i].NotNull)
	}
}

package wal

import (
	"testing"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"gotest.tools/v3/assert"
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
	assert.Assert(t, len(data) > 0)

	// Decode
	decoded, err := DecodeTableSchema(data)
	assert.NilError(t, err)

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

func TestSchemaEncoder_EmptyColumns(t *testing.T) {
	s := &schema.TableSchema{
		TableName: "empty_table",
		Columns:   []schema.Column{},
	}

	// Encode
	data := EncodeTableSchema(s)
	assert.Assert(t, len(data) > 0)

	// Decode
	decoded, err := DecodeTableSchema(data)
	assert.NilError(t, err)

	assert.Equal(t, s.TableName, decoded.TableName)
	assert.Equal(t, 0, len(decoded.Columns))
}

func TestSchemaEncoder_LongNames(t *testing.T) {
	longName := ""
	for i := 0; i < 300; i++ {
		longName += "a"
	}

	s := &schema.TableSchema{
		TableName: longName,
		Columns: []schema.Column{
			{Name: longName + "_col", Type: "TEXT"},
		},
	}

	// Encode
	data := EncodeTableSchema(s)
	assert.Assert(t, len(data) > 0)

	// Decode
	decoded, err := DecodeTableSchema(data)
	assert.NilError(t, err)

	assert.Equal(t, s.TableName, decoded.TableName)
	assert.Equal(t, 1, len(decoded.Columns))
	assert.Equal(t, s.Columns[0].Name, decoded.Columns[0].Name)
	assert.Equal(t, s.Columns[0].Type, decoded.Columns[0].Type)
}

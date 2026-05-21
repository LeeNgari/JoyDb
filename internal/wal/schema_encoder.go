package wal

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
)

// ===========================================================================
// SCHEMA BINARY ENCODER
// ===========================================================================
//
// Binary format for TableSchema:
//   [NameLen(2)][TableName(N)] [ColCount(2)] [Col1] [Col2] ...
//
// Binary format for a single Column:
//   [NameLen(2)][Name(N)][Type(1)][Flags(1)]
//
// Column Type byte:
//   0x01=INT, 0x02=FLOAT, 0x03=TEXT, 0x04=BOOL,
//   0x05=DATE, 0x06=TIME, 0x07=EMAIL
//
// Column Flags byte:
//   bit0=PrimaryKey, bit1=Unique, bit2=NotNull, bit3=AutoIncrement
//
// This format is also used by the Phase 3 snapshot system, keeping the
// schema representation consistent across the entire persistence layer.
// ===========================================================================

// columnTypeToByte maps ColumnType to a single byte identifier
var columnTypeToByte = map[schema.ColumnType]byte{
	schema.ColumnTypeInt:   0x01,
	schema.ColumnTypeFloat: 0x02,
	schema.ColumnTypeText:  0x03,
	schema.ColumnTypeBool:  0x04,
	schema.ColumnTypeDate:  0x05,
	schema.ColumnTypeTime:  0x06,
	schema.ColumnTypeEmail: 0x07,
}

// byteToColumnType maps a byte identifier back to ColumnType
var byteToColumnType = map[byte]schema.ColumnType{
	0x01: schema.ColumnTypeInt,
	0x02: schema.ColumnTypeFloat,
	0x03: schema.ColumnTypeText,
	0x04: schema.ColumnTypeBool,
	0x05: schema.ColumnTypeDate,
	0x06: schema.ColumnTypeTime,
	0x07: schema.ColumnTypeEmail,
}

// EncodeTableSchema encodes a TableSchema to a binary byte slice.
func EncodeTableSchema(s *schema.TableSchema) []byte {
	// Pre-calculate size
	size := 2 + len(s.TableName) + 2 // NameLen + Name + ColCount
	for _, col := range s.Columns {
		size += encodedColumnSize(col)
	}
	size += 2 // FKCount
	for _, fk := range s.ForeignKeys {
		size += encodedForeignKeySize(fk)
	}

	buf := make([]byte, size)
	offset := 0

	// TableName (2 bytes len + string)
	ByteOrder.PutUint16(buf[offset:], uint16(len(s.TableName)))
	offset += 2
	copy(buf[offset:], s.TableName)
	offset += len(s.TableName)

	// ColCount (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(s.Columns)))
	offset += 2

	// Columns
	for _, col := range s.Columns {
		n := encodeColumnInto(col, buf[offset:])
		offset += n
	}

	// FKCount (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(s.ForeignKeys)))
	offset += 2

	// ForeignKeys
	for _, fk := range s.ForeignKeys {
		n := encodeForeignKeyInto(fk, buf[offset:])
		offset += n
	}

	return buf
}

// DecodeTableSchema decodes a binary byte slice into a TableSchema.
func DecodeTableSchema(data []byte) (*schema.TableSchema, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("schema data too short: %d bytes", len(data))
	}

	offset := 0

	// TableName
	nameLen := int(ByteOrder.Uint16(data[offset:]))
	offset += 2
	if offset+nameLen > len(data) {
		return nil, fmt.Errorf("schema buffer too small for table name")
	}
	tableName := string(data[offset : offset+nameLen])
	offset += nameLen

	// ColCount (2 bytes)
	if offset+2 > len(data) {
		return nil, fmt.Errorf("schema buffer too small for column count")
	}
	colCount := int(ByteOrder.Uint16(data[offset:]))
	offset += 2

	columns := make([]schema.Column, 0, colCount)
	for i := 0; i < colCount; i++ {
		col, newOffset, err := decodeColumnFrom(data, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode column %d: %w", i, err)
		}
		columns = append(columns, col)
		offset = newOffset
	}

	// Read ForeignKeys if they are present in the encoded bytes (backwards-compatible)
	var fks []schema.ForeignKey
	if offset+2 <= len(data) {
		fkCount := int(ByteOrder.Uint16(data[offset:]))
		offset += 2
		fks = make([]schema.ForeignKey, 0, fkCount)
		for i := 0; i < fkCount; i++ {
			fk, newOffset, err := decodeForeignKeyFrom(data, offset)
			if err != nil {
				return nil, fmt.Errorf("failed to decode foreign key %d: %w", i, err)
			}
			fks = append(fks, fk)
			offset = newOffset
		}
	}

	return &schema.TableSchema{TableName: tableName, Columns: columns, ForeignKeys: fks}, nil
}

// EncodeColumn encodes a single Column to a binary byte slice.
// Used by AlterTableRecord payloads.
func EncodeColumn(col schema.Column) []byte {
	buf := make([]byte, encodedColumnSize(col))
	encodeColumnInto(col, buf)
	return buf
}

// DecodeColumn decodes a single Column from a binary byte slice at the given offset.
// Returns the decoded Column, the new offset after the column data, and any error.
func DecodeColumn(data []byte, offset int) (schema.Column, int, error) {
	return decodeColumnFrom(data, offset)
}

// ===========================================================================
// INTERNAL HELPERS
// ===========================================================================

// encodedColumnSize returns the byte size of an encoded column
func encodedColumnSize(col schema.Column) int {
	return 2 + len(col.Name) + 1 + 1 // NameLen(2) + Name + Type(1) + Flags(1)
}

// encodeColumnInto encodes a column into dst starting at offset 0.
// Returns the number of bytes written.
func encodeColumnInto(col schema.Column, dst []byte) int {
	offset := 0

	// NameLen (2 bytes) + Name
	ByteOrder.PutUint16(dst[offset:], uint16(len(col.Name)))
	offset += 2
	copy(dst[offset:], col.Name)
	offset += len(col.Name)

	// Type (1 byte)
	typeByte, ok := columnTypeToByte[col.Type]
	if !ok {
		typeByte = 0x03 // default to TEXT for unknown types
	}
	dst[offset] = typeByte
	offset++

	// Flags (1 byte): bit0=PK, bit1=Unique, bit2=NotNull, bit3=AutoIncrement
	var flags byte
	if col.PrimaryKey {
		flags |= 0x01
	}
	if col.Unique {
		flags |= 0x02
	}
	if col.NotNull {
		flags |= 0x04
	}
	if col.AutoIncrement {
		flags |= 0x08
	}
	dst[offset] = flags
	offset++

	return offset
}

// decodeColumnFrom decodes a single column from data at the given offset.
// Returns the column, the new offset, and any error.
func decodeColumnFrom(data []byte, offset int) (schema.Column, int, error) {
	if offset+2 > len(data) {
		return schema.Column{}, 0, fmt.Errorf("not enough data for column name length at offset %d", offset)
	}

	// NameLen + Name
	nameLen := int(ByteOrder.Uint16(data[offset:]))
	offset += 2

	if offset+nameLen > len(data) {
		return schema.Column{}, 0, fmt.Errorf("not enough data for column name of length %d at offset %d", nameLen, offset)
	}
	name := string(data[offset : offset+nameLen])
	offset += nameLen

	// Type (1 byte)
	if offset+2 > len(data) {
		return schema.Column{}, 0, fmt.Errorf("not enough data for column type/flags at offset %d", offset)
	}
	colType, ok := byteToColumnType[data[offset]]
	if !ok {
		return schema.Column{}, 0, fmt.Errorf("unknown column type byte 0x%02x at offset %d", data[offset], offset)
	}
	offset++

	// Flags (1 byte)
	flags := data[offset]
	offset++

	col := schema.Column{
		Name:          name,
		Type:          colType,
		PrimaryKey:    flags&0x01 != 0,
		Unique:        flags&0x02 != 0,
		NotNull:       flags&0x04 != 0,
		AutoIncrement: flags&0x08 != 0,
	}

	return col, offset, nil
}

// encodedForeignKeySize returns the byte size of an encoded foreign key
func encodedForeignKeySize(fk schema.ForeignKey) int {
	return 2 + len(fk.ColumnName) + 2 + len(fk.RefTableName) + 2 + len(fk.RefColumnName)
}

// encodeForeignKeyInto encodes a foreign key into dst starting at offset 0.
// Returns the number of bytes written.
func encodeForeignKeyInto(fk schema.ForeignKey, dst []byte) int {
	offset := 0

	// ColumnName
	ByteOrder.PutUint16(dst[offset:], uint16(len(fk.ColumnName)))
	offset += 2
	copy(dst[offset:], fk.ColumnName)
	offset += len(fk.ColumnName)

	// RefTableName
	ByteOrder.PutUint16(dst[offset:], uint16(len(fk.RefTableName)))
	offset += 2
	copy(dst[offset:], fk.RefTableName)
	offset += len(fk.RefTableName)

	// RefColumnName
	ByteOrder.PutUint16(dst[offset:], uint16(len(fk.RefColumnName)))
	offset += 2
	copy(dst[offset:], fk.RefColumnName)
	offset += len(fk.RefColumnName)

	return offset
}

// decodeForeignKeyFrom decodes a single foreign key from data at the given offset.
// Returns the foreign key, the new offset, and any error.
func decodeForeignKeyFrom(data []byte, offset int) (schema.ForeignKey, int, error) {
	if offset+2 > len(data) {
		return schema.ForeignKey{}, 0, fmt.Errorf("not enough data for columnName length at offset %d", offset)
	}
	colNameLen := int(ByteOrder.Uint16(data[offset:]))
	offset += 2
	if offset+colNameLen > len(data) {
		return schema.ForeignKey{}, 0, fmt.Errorf("not enough data for columnName of length %d", colNameLen)
	}
	colName := string(data[offset : offset+colNameLen])
	offset += colNameLen

	if offset+2 > len(data) {
		return schema.ForeignKey{}, 0, fmt.Errorf("not enough data for refTableName length at offset %d", offset)
	}
	refTableLen := int(ByteOrder.Uint16(data[offset:]))
	offset += 2
	if offset+refTableLen > len(data) {
		return schema.ForeignKey{}, 0, fmt.Errorf("not enough data for refTableName of length %d", refTableLen)
	}
	refTableName := string(data[offset : offset+refTableLen])
	offset += refTableLen

	if offset+2 > len(data) {
		return schema.ForeignKey{}, 0, fmt.Errorf("not enough data for refColumnName length at offset %d", offset)
	}
	refColLen := int(ByteOrder.Uint16(data[offset:]))
	offset += 2
	if offset+refColLen > len(data) {
		return schema.ForeignKey{}, 0, fmt.Errorf("not enough data for refColumnName of length %d", refColLen)
	}
	refColName := string(data[offset : offset+refColLen])
	offset += refColLen

	fk := schema.ForeignKey{
		ColumnName:    colName,
		RefTableName:  refTableName,
		RefColumnName: refColName,
	}
	return fk, offset, nil
}

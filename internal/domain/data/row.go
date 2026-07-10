package data

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

const (
	typeTagInt64   byte = 1
	typeTagFloat64 byte = 2
	typeTagString  byte = 3
	typeTagBool    byte = 4
	typeTagNull    byte = 5
)

// Row represents a single table row
// Values = cell values ordered sequentially matching the table schema
type Row struct {
	Values  []interface{}
	RID     int64 // Immutable Record ID assigned on insert
	Deleted bool  // Tombstone flag — true means logically deleted
}

// NewRow creates a new Row with the given values array
func NewRow(values []interface{}) Row {
	return Row{
		Values:  values,
		RID:     0,
		Deleted: false,
	}
}

// Copy creates a deep copy of the row to prevent mutation
func (r Row) Copy() Row {
	copied := make([]interface{}, len(r.Values))
	copy(copied, r.Values)
	return Row{
		Values:  copied,
		RID:     r.RID,
		Deleted: r.Deleted,
	}
}

// UnmarshalJSON implements json.Unmarshaler interface
// This allows Row to be unmarshaled from JSON as an array
func (r *Row) UnmarshalJSON(data []byte) error {
	var m []interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	r.Values = m
	return nil
}

// MarshalJSON implements json.Marshaler interface
// This allows Row to be marshaled to JSON as an array
func (r Row) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Values)
}

// Serialize serializes the row to []byte using a custom binary format.
func (r Row) Serialize() ([]byte, error) {
	// Pre-calculate size
	size := 9 + 2 // 8 bytes RID + 1 byte Deleted + 2 bytes FieldCount
	for _, v := range r.Values {
		size += 1 // TypeTag
		size += 4 // ValLen
		switch val := v.(type) {
		case int, int64, float64:
			size += 8
		case string:
			size += len(val)
		case bool:
			size += 1
		case nil:
			size += 0
		default:
			return nil, fmt.Errorf("unsupported type in row serialization: %T", v)
		}
	}

	buf := make([]byte, size)
	offset := 0

	// RID (8 bytes)
	binary.LittleEndian.PutUint64(buf[offset:], uint64(r.RID))
	offset += 8

	// Deleted (1 byte)
	if r.Deleted {
		buf[offset] = 1
	} else {
		buf[offset] = 0
	}
	offset++

	// FieldCount (2 bytes)
	binary.LittleEndian.PutUint16(buf[offset:], uint16(len(r.Values)))
	offset += 2

	for _, v := range r.Values {
		// TypeTag, ValLen, Val
		switch val := v.(type) {
		case int:
			buf[offset] = typeTagInt64
			offset++
			binary.LittleEndian.PutUint32(buf[offset:], 8)
			offset += 4
			binary.LittleEndian.PutUint64(buf[offset:], uint64(int64(val)))
			offset += 8
		case int64:
			buf[offset] = typeTagInt64
			offset++
			binary.LittleEndian.PutUint32(buf[offset:], 8)
			offset += 4
			binary.LittleEndian.PutUint64(buf[offset:], uint64(val))
			offset += 8
		case float64:
			buf[offset] = typeTagFloat64
			offset++
			binary.LittleEndian.PutUint32(buf[offset:], 8)
			offset += 4
			binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(val))
			offset += 8
		case string:
			buf[offset] = typeTagString
			offset++
			binary.LittleEndian.PutUint32(buf[offset:], uint32(len(val)))
			offset += 4
			copy(buf[offset:], val)
			offset += len(val)
		case bool:
			buf[offset] = typeTagBool
			offset++
			binary.LittleEndian.PutUint32(buf[offset:], 1)
			offset += 4
			if val {
				buf[offset] = 1
			} else {
				buf[offset] = 0
			}
			offset += 1
		case nil:
			buf[offset] = typeTagNull
			offset++
			binary.LittleEndian.PutUint32(buf[offset:], 0)
			offset += 4
		}
	}

	return buf, nil
}

// Deserialize creates a Row from []byte using the custom binary format.
func Deserialize(data []byte) (Row, error) {
	if len(data) < 11 {
		return Row{}, fmt.Errorf("data too short for row deserialization")
	}

	offset := 0

	// RID (8 bytes)
	rid := int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	// Deleted (1 byte)
	deleted := data[offset] == 1
	offset++

	fieldCount := int(binary.LittleEndian.Uint16(data[offset:]))
	offset += 2

	m := make([]interface{}, fieldCount)

	for i := 0; i < fieldCount; i++ {
		if offset+5 > len(data) {
			return Row{}, fmt.Errorf("data too short for type tag and value length")
		}
		typeTag := data[offset]
		offset++

		valLen := int(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4

		if offset+valLen > len(data) {
			return Row{}, fmt.Errorf("data too short for value")
		}

		switch typeTag {
		case typeTagInt64:
			if valLen != 8 {
				return Row{}, fmt.Errorf("invalid value length for int64: %d", valLen)
			}
			val := int64(binary.LittleEndian.Uint64(data[offset:]))
			m[i] = val
		case typeTagFloat64:
			if valLen != 8 {
				return Row{}, fmt.Errorf("invalid value length for float64: %d", valLen)
			}
			val := math.Float64frombits(binary.LittleEndian.Uint64(data[offset:]))
			m[i] = val
		case typeTagString:
			m[i] = string(data[offset : offset+valLen])
		case typeTagBool:
			if valLen != 1 {
				return Row{}, fmt.Errorf("invalid value length for bool: %d", valLen)
			}
			m[i] = data[offset] == 1
		case typeTagNull:
			m[i] = nil
		default:
			return Row{}, fmt.Errorf("unknown type tag: %d", typeTag)
		}
		offset += valLen
	}

	row := NewRow(m)
	row.RID = rid
	row.Deleted = deleted
	return row, nil
}

package data

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

const (
	typeTagInt64   byte = 1
	typeTagFloat64 byte = 2
	typeTagString  byte = 3
	typeTagBool    byte = 4
	typeTagNull    byte = 5
)

// Row represents a single table row
// Key = column name, Value = cell value
type Row struct {
	Data map[string]interface{}
	// mu is a placeholder for future row-level locking implementation
	// Currently unused but reserved for fine-grained concurrency control
	mu *sync.Mutex
}

// NewRow creates a new Row with the given data
func NewRow(data map[string]interface{}) Row {
	return Row{
		Data: data,
		mu:   &sync.Mutex{},
	}
}

// Copy creates a deep copy of the row to prevent mutation
func (r Row) Copy() Row {
	copy := make(map[string]interface{}, len(r.Data))
	for k, v := range r.Data {
		copy[k] = v
	}
	return Row{
		Data: copy,
		mu:   &sync.Mutex{},
	}
}

// UnmarshalJSON implements json.Unmarshaler interface
// This allows Row to be unmarshaled from JSON as a map
func (r *Row) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	r.Data = m
	return nil
}

// MarshalJSON implements json.Marshaler interface
// This allows Row to be marshaled to JSON as a map
func (r Row) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

// Serialize serializes the row to []byte using a custom binary format.
func (r Row) Serialize() ([]byte, error) {
	// Pre-calculate size
	size := 2 // FieldCount
	for k, v := range r.Data {
		size += 2 + len(k) // KeyLen + Key
		size += 1          // TypeTag
		size += 4          // ValLen
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

	// FieldCount (2 bytes)
	binary.LittleEndian.PutUint16(buf[offset:], uint16(len(r.Data)))
	offset += 2

	for k, v := range r.Data {
		// KeyLen + Key
		binary.LittleEndian.PutUint16(buf[offset:], uint16(len(k)))
		offset += 2
		copy(buf[offset:], k)
		offset += len(k)

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
	if len(data) < 2 {
		return Row{}, fmt.Errorf("data too short for row deserialization")
	}

	offset := 0
	fieldCount := int(binary.LittleEndian.Uint16(data[offset:]))
	offset += 2

	m := make(map[string]interface{}, fieldCount)

	for i := 0; i < fieldCount; i++ {
		if offset+2 > len(data) {
			return Row{}, fmt.Errorf("data too short for key length")
		}
		keyLen := int(binary.LittleEndian.Uint16(data[offset:]))
		offset += 2

		if offset+keyLen > len(data) {
			return Row{}, fmt.Errorf("data too short for key string")
		}
		key := string(data[offset : offset+keyLen])
		offset += keyLen

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
			m[key] = val
		case typeTagFloat64:
			if valLen != 8 {
				return Row{}, fmt.Errorf("invalid value length for float64: %d", valLen)
			}
			val := math.Float64frombits(binary.LittleEndian.Uint64(data[offset:]))
			m[key] = val
		case typeTagString:
			m[key] = string(data[offset : offset+valLen])
		case typeTagBool:
			if valLen != 1 {
				return Row{}, fmt.Errorf("invalid value length for bool: %d", valLen)
			}
			m[key] = data[offset] == 1
		case typeTagNull:
			m[key] = nil
		default:
			return Row{}, fmt.Errorf("unknown type tag: %d", typeTag)
		}
		offset += valLen
	}

	return NewRow(m), nil
}

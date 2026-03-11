package data

import (
	"encoding/json"
	"sync"
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

// ToJSON serializes the row to json.RawMessage for WAL integration
func (r Row) ToJSON() (json.RawMessage, error) {
	return json.Marshal(r.Data)
}

// FromJSON creates a Row from json.RawMessage (for WAL recovery)
func FromJSON(data json.RawMessage) (Row, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return Row{}, err
	}
	return NewRow(m), nil
}

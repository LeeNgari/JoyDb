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
	mu sync.Mutex
}

// NewRow creates a new Row with the given data
func NewRow(data map[string]interface{}) Row {
	return Row{
		Data: data,
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
	}
}

// MarshalJSON implements custom JSON marshaling to flatten the structure
// This ensures the row appears as a simple map {"id": 1, ...} instead of {"Data": {"id": 1, ...}}
func (r Row) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

// UnmarshalJSON implements custom JSON unmarshaling to handle the flat structure
func (r *Row) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	r.Data = m
	return nil
}

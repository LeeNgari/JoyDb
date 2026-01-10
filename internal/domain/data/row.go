package data

// Row represents a single table row
// Key = column name, Value = cell value
type Row map[string]interface{}

// Copy creates a deep copy of the row to prevent mutation
func (r Row) Copy() Row {
	copy := make(Row, len(r))
	for k, v := range r {
		copy[k] = v
	}
	return copy
}

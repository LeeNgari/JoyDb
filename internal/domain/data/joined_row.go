package data

import "fmt"

// JoinedRow represents a row that combines data from multiple tables
// Column names are qualified with table names (e.g., "users.id", "orders.product")
type JoinedRow struct {
	Data map[string]interface{}
}

// NewJoinedRow creates a new JoinedRow with initialized data map
func NewJoinedRow() JoinedRow {
	return JoinedRow{
		Data: make(map[string]interface{}),
	}
}

// Get retrieves a value by qualified column name (e.g., "users.id")
func (jr JoinedRow) Get(qualifiedName string) (interface{}, bool) {
	val, exists := jr.Data[qualifiedName]
	return val, exists
}

// Set adds or updates a value with qualified column name
func (jr JoinedRow) Set(qualifiedName string, value interface{}) {
	jr.Data[qualifiedName] = value
}

// String returns a string representation for debugging
func (jr JoinedRow) String() string {
	return fmt.Sprintf("JoinedRow%v", jr.Data)
}

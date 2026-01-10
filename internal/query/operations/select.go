package operations

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
)

// SelectAll returns all rows of the table
func SelectAll(table *schema.Table) []data.Row {
	table.RLock()
	defer table.RUnlock()

	rows := make([]data.Row, len(table.Rows))
	copy(rows, table.Rows)
	return rows
}

// PredicateFunc is a function that tests whether a row matches certain criteria
type PredicateFunc func(data.Row) bool

// SelectWhere returns rows that match the given predicate
func SelectWhere(table *schema.Table, pred PredicateFunc) []data.Row {
	table.RLock()
	defer table.RUnlock()

	var result []data.Row
	for _, row := range table.Rows {
		if pred(row) {
			result = append(result, row)
		}
	}
	return result
}

// SelectByUniqueIndex retrieves a row using a unique index
func SelectByUniqueIndex(table *schema.Table, colName string, value interface{}) (data.Row, bool) {
	table.RLock()
	defer table.RUnlock()

	idx, exists := table.Indexes[colName]
	if !exists || !idx.Unique {
		return nil, false
	}

	// Convert value to int64 if it's an integer type for comparison
	if intVal, ok := value.(int); ok {
		value = int64(intVal)
	}

	positions, found := idx.Data[value]
	if !found || len(positions) == 0 {
		return nil, false
	}

	return table.Rows[positions[0]], true
}

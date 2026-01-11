package operations

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/errors"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/query/validation"
)

// Update modifies rows matching the predicate and applies the given updates.
// Returns number of rows updated and any error.
// On constraint violation during index update, rolls back the index change.
func Update(table *schema.Table, pred PredicateFunc, updates data.Row) (int, error) {
	// Acquire write lock for the entire operation
	table.Lock()
	defer table.Unlock()

	updated := 0

	for i := range table.Rows {
		oldRow := table.Rows[i]

		if !pred(oldRow) {
			continue
		}

		// Create proposed new row (merge old + updates)
		newRow := oldRow.Copy()
		for k, v := range updates {
			newRow[k] = v
		}

		// Validate new row
		if err := validation.ValidateRow(table, newRow, i); err != nil {
			return updated, fmt.Errorf("validation failed for row %d: %w", i, err)
		}

		// Check for unique constraint violations on updated columns
		for colName, idx := range table.Indexes {
			if !idx.Unique {
				continue
			}

			// Check if this column was updated
			newVal, newExists := newRow[colName]
			oldVal, oldExists := oldRow[colName]

			// Skip if column wasn't updated or value didn't change
			if !newExists || (oldExists && newVal == oldVal) {
				continue
			}

			// Check if new value already exists in another row
			if positions, found := idx.Data[newVal]; found {
				for _, pos := range positions {
					if pos != i { // Different row has this value
						col := getColumnByName(table, colName)
						if col != nil && col.PrimaryKey {
							return updated, errors.NewPrimaryKeyViolation(table.Name, colName, newVal)
						}
						return updated, errors.NewUniqueViolation(table.Name, colName, newVal, positions)
					}
				}
			}
		}

		// Remove old index entries
		removeFromIndexes(table, oldRow, i)

		// Add new index entries
		addToIndexes(table, newRow, i)

		// Apply update
		table.Rows[i] = newRow
		updated++
	}

	if updated > 0 {
		table.MarkDirtyUnsafe()
	}

	return updated, nil
}

// UpdateByID updates a single row by its primary key ID
func UpdateByID(table *schema.Table, id interface{}, updates data.Row) error {
	count, err := Update(table, func(r data.Row) bool {
		pkCol := table.Schema.GetPrimaryKeyColumn()
		if pkCol == nil {
			return false
		}
		return r[pkCol.Name] == id
	}, updates)

	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("no row found with id %v", id)
	}

	return nil
}

// removeFromIndexes removes a row's position from all relevant indexes
func removeFromIndexes(table *schema.Table, row data.Row, rowPos int) {
	for colName, idx := range table.Indexes {
		val, exists := row[colName]
		if !exists {
			continue
		}

		positions, found := idx.Data[val]
		if !found {
			continue
		}

		// Filter out the position
		newPositions := make([]int, 0, len(positions)-1)
		for _, p := range positions {
			if p != rowPos {
				newPositions = append(newPositions, p)
			}
		}

		if len(newPositions) == 0 {
			delete(idx.Data, val) // Clean up empty entries
		} else {
			idx.Data[val] = newPositions
		}
	}
}

// addToIndexes adds a row's position to all relevant indexes
func addToIndexes(table *schema.Table, row data.Row, rowPos int) {
	for colName, idx := range table.Indexes {
		val, exists := row[colName]
		if !exists {
			continue
		}

		idx.Data[val] = append(idx.Data[val], rowPos)
	}
}

// getColumnByName finds a column by name in the table schema
func getColumnByName(table *schema.Table, name string) *schema.Column {
	for i := range table.Schema.Columns {
		if table.Schema.Columns[i].Name == name {
			return &table.Schema.Columns[i]
		}
	}
	return nil
}

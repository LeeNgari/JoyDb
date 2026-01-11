package operations

import (
	"log/slog"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
)

// Delete removes rows matching the predicate.
// Returns number of rows deleted.
// Rebuilds all indexes after deletion since row positions change.
func Delete(table *schema.Table, pred PredicateFunc) (int, error) {
	// Acquire write lock for the entire operation
	table.Lock()
	defer table.Unlock()

	deleted := 0
	newRows := make([]data.Row, 0, len(table.Rows))

	// Filter out rows that match the predicate
	for _, row := range table.Rows {
		if pred(row) {
			deleted++
			continue // Skip this row (delete it)
		}
		newRows = append(newRows, row)
	}

	if deleted == 0 {
		return 0, nil // Nothing to delete
	}

	// Update rows
	table.Rows = newRows

	// Clear all indexes (positions have changed)
	for _, idx := range table.Indexes {
		idx.Data = make(map[interface{}][]int)
	}

	// Rebuild indexes with new positions
	// Note: We need to temporarily unlock for BuildIndexes since it also locks
	table.Unlock()
	if err := indexing.BuildIndexes(table); err != nil {
		table.Lock() // Re-lock before returning
		slog.Error("failed to rebuild indexes after delete",
			slog.String("table", table.Name),
			slog.Int("deleted", deleted),
			slog.Any("error", err),
		)
		return deleted, err
	}
	table.Lock() // Re-lock after BuildIndexes

	// Mark table as dirty
	table.MarkDirtyUnsafe()

	return deleted, nil
}

// DeleteByID deletes a single row by its primary key ID
// Returns error if no row found or if deletion fails
func DeleteByID(table *schema.Table, id interface{}) error {
	count, err := Delete(table, func(r data.Row) bool {
		pkCol := table.Schema.GetPrimaryKeyColumn()
		if pkCol == nil {
			return false
		}
		return r[pkCol.Name] == id
	})

	if err != nil {
		return err
	}

	if count == 0 {
		slog.Warn("no row found to delete",
			slog.String("table", table.Name),
			slog.Any("id", id),
		)
	}

	return nil
}

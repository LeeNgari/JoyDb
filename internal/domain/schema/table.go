package schema

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/errors"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
)

// Table represents a database table with its schema, data, and indexes
type Table struct {
	mu           sync.RWMutex
	Name         string
	Path         string // filesystem path to table directory
	Schema       *TableSchema
	Rows         []data.Row
	Indexes      map[string]*data.Index
	LastInsertID int64
	Dirty        bool // tracks if table has unsaved changes
}

// MarkDirty marks the table as having unsaved changes
// This should be called after any mutation operation (INSERT, UPDATE, DELETE)
func (t *Table) MarkDirty() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.MarkDirtyUnsafe()
}

// MarkDirtyUnsafe sets dirty flag without acquiring lock
// IMPORTANT: Only call this when you already hold the table lock!
// Use MarkDirty() if you don't hold the lock.
func (t *Table) MarkDirtyUnsafe() {
	t.Dirty = true
}

// Lock acquires an exclusive lock on the table for write operations
func (t *Table) Lock() {
	t.mu.Lock()
}

// Unlock releases the exclusive lock
func (t *Table) Unlock() {
	t.mu.Unlock()
}

// RLock acquires a read lock on the table for read operations
func (t *Table) RLock() {
	t.mu.RLock()
}

// RUnlock releases the read lock
func (t *Table) RUnlock() {
	t.mu.RUnlock()
}

// Insert adds a new row to the table with full validation and auto-increment support
func (t *Table) Insert(mutRow data.Row, tx *transaction.Transaction) error {
	row := mutRow.Copy() // prevent mutation of caller's data

	// Acquire write lock for the entire operation
	t.Lock()
	defer t.Unlock()

	if tx != nil {
		slog.Debug("Insert operation", "table", t.Name, "tx_id", tx.ID)
	}

	// 1. Handle auto-increment primary key FIRST (before validation)
	var autoIncCol *Column
	for _, col := range t.Schema.Columns {
		if col.AutoIncrement && col.PrimaryKey {
			autoIncCol = &col
			break
		}
	}

	if autoIncCol != nil {
		// Generate next ID
		nextID := t.LastInsertID + 1

		// Allow user to override auto-increment
		if val, exists := row[autoIncCol.Name]; exists {
			userID, ok := normalizeToInt64(val)
			if !ok {
				return &errors.ConstraintError{
					Table:      t.Name,
					Column:     autoIncCol.Name,
					Value:      val,
					Constraint: "auto_increment",
					Reason:     "auto-increment column must be integer",
				}
			}
			// Prevent sequence conflicts
			if userID <= t.LastInsertID {
				return &errors.ConstraintError{
					Table:      t.Name,
					Column:     autoIncCol.Name,
					Value:      userID,
					Constraint: "auto_increment",
					Reason:     "provided value is not greater than current sequence",
				}
			}
			nextID = userID
		}

		// Set the auto-increment value
		row[autoIncCol.Name] = nextID
		t.LastInsertID = nextID
	} else {
		// If PK is not auto-increment, it must be provided
		pkCol := t.Schema.GetPrimaryKeyColumn()
		if pkCol != nil {
			if _, exists := row[pkCol.Name]; !exists {
				return &errors.ConstraintError{
					Table:      t.Name,
					Column:     pkCol.Name,
					Constraint: "primary_key",
					Reason:     "primary key value required",
				}
			}
		}
	}

	// 2. Validate the row (types, NOT NULL, etc.)
	if err := t.validateRow(row); err != nil {
		return err
	}

	// 3. Check unique/primary constraints using current indexes
	for colName, idx := range t.Indexes {
		val, exists := row[colName]
		if !exists {
			continue
		}

		if idx.Unique {
			if _, found := idx.Data[val]; found {
				return &errors.ConstraintError{
					Table:      t.Name,
					Column:     colName,
					Value:      val,
					Constraint: "unique",
					Reason:     "duplicate value",
				}
			}
		}
	}

	// 4. Get new position (BEFORE append)
	newRowPos := len(t.Rows)

	// 5. Everything passed → safe to append
	t.Rows = append(t.Rows, row)

	// 6. Update all indexes
	for colName, idx := range t.Indexes {
		if val, exists := row[colName]; exists {
			idx.Data[val] = append(idx.Data[val], newRowPos)
		}
	}

	// 7. Mark table as dirty (has unsaved changes)
	t.MarkDirtyUnsafe()

	return nil
}

// SelectAll returns all rows of the table
func (t *Table) SelectAll(tx *transaction.Transaction) []data.Row {
	t.RLock()
	defer t.RUnlock()

	if tx != nil {
		slog.Debug("SelectAll operation", "table", t.Name, "tx_id", tx.ID)
	}

	rows := make([]data.Row, len(t.Rows))
	copy(rows, t.Rows)
	return rows
}

// Select returns rows that match the given predicate
func (t *Table) Select(predicate func(data.Row) bool, tx *transaction.Transaction) []data.Row {
	t.RLock()
	defer t.RUnlock()

	if tx != nil {
		slog.Debug("Select operation", "table", t.Name, "tx_id", tx.ID)
	}

	var result []data.Row
	for _, row := range t.Rows {
		if predicate(row) {
			result = append(result, row)
		}
	}
	return result
}

// SelectByIndex retrieves a row using a unique index
// Returns the row and true if found, nil and false otherwise
func (t *Table) SelectByIndex(colName string, value interface{}, tx *transaction.Transaction) (data.Row, bool) {
	t.RLock()
	defer t.RUnlock()

	if tx != nil {
		slog.Debug("SelectByIndex operation", "table", t.Name, "column", colName, "tx_id", tx.ID)
	}

	idx, exists := t.Indexes[colName]
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

	return t.Rows[positions[0]], true
}

// Update modifies rows that match the given predicate
// Returns the number of rows updated
func (t *Table) Update(predicate func(data.Row) bool, updates data.Row, tx *transaction.Transaction) (int, error) {
	t.Lock()
	defer t.Unlock()

	if tx != nil {
		slog.Debug("Update operation", "table", t.Name, "tx_id", tx.ID)
	}

	count := 0
	for i, row := range t.Rows {
		if predicate(row) {
			// Validate each update value against schema
			for colName, newValue := range updates {
				// Find column in schema
				var col *Column
				for i := range t.Schema.Columns {
					if t.Schema.Columns[i].Name == colName {
						col = &t.Schema.Columns[i]
						break
					}
				}
				if col == nil {
					return 0, &errors.ColumnNotFoundError{
						TableName:  t.Name,
						ColumnName: colName,
					}
				}

				// Type validation would go here if needed
				t.Rows[i][colName] = newValue
			}
			count++
		}
	}

	if count > 0 {
		// Rebuild indexes after update
		t.rebuildIndexesUnsafe()
		t.MarkDirtyUnsafe()
	}

	return count, nil
}

// Delete removes rows that match the given predicate
// Returns the number of rows deleted
func (t *Table) Delete(predicate func(data.Row) bool, tx *transaction.Transaction) (int, error) {
	t.Lock()
	defer t.Unlock()

	if tx != nil {
		slog.Debug("Delete operation", "table", t.Name, "tx_id", tx.ID)
	}

	var newRows []data.Row
	deleted := 0

	for _, row := range t.Rows {
		if predicate(row) {
			deleted++
		} else {
			newRows = append(newRows, row)
		}
	}

	if deleted > 0 {
		t.Rows = newRows
		t.rebuildIndexesUnsafe()
		t.MarkDirtyUnsafe()
	}

	return deleted, nil
}

// validateRow validates a row against the table schema
// Must be called while holding a lock
func (t *Table) validateRow(row data.Row) error {
	for _, col := range t.Schema.Columns {
		value, exists := row[col.Name]

		// Check NOT NULL constraint
		if col.NotNull && !exists {
			return &errors.ConstraintError{
				Table:      t.Name,
				Column:     col.Name,
				Constraint: "not_null",
				Reason:     "missing required value",
			}
		}

		// Skip type validation if value doesn't exist
		if !exists {
			continue
		}

		// Type validation
		if err := t.validateType(col.Name, value, col.Type); err != nil {
			return err
		}
	}
	return nil
}

// validateType validates that a value matches the expected column type
func (t *Table) validateType(colName string, value interface{}, expectedType ColumnType) error {
	switch expectedType {
	case ColumnTypeInt:
		if _, ok := value.(int64); !ok {
			if _, ok := value.(int); !ok {
				return fmt.Errorf("column %s: expected INT, got %T", colName, value)
			}
		}
	case ColumnTypeFloat:
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("column %s: expected FLOAT, got %T", colName, value)
		}
	case ColumnTypeText:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("column %s: expected TEXT, got %T", colName, value)
		}
	case ColumnTypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("column %s: expected BOOL, got %T", colName, value)
		}
	}
	return nil
}

// rebuildIndexesUnsafe rebuilds all indexes
// IMPORTANT: Must be called while holding write lock!
func (t *Table) rebuildIndexesUnsafe() {
	// Clear existing indexes
	for _, idx := range t.Indexes {
		idx.Data = make(map[interface{}][]int)
	}

	// Rebuild from current rows
	for rowPos, row := range t.Rows {
		for colName, idx := range t.Indexes {
			if val, exists := row[colName]; exists {
				idx.Data[val] = append(idx.Data[val], rowPos)
			}
		}
	}
}

// normalizeToInt64 converts various numeric types to int64
// Returns the int64 value and true if successful, 0 and false otherwise
func normalizeToInt64(val interface{}) (int64, bool) {
	switch v := val.(type) {
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}

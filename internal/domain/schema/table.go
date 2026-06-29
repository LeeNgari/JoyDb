package schema

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/errors"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/index/btree"
)

// Table represents a database table with its schema, data, and indexes
type Table struct {
	mu             sync.RWMutex
	Name           string
	Path           string // database directory path (used for WAL/snapshot context)
	Schema         *TableSchema
	RowsByRID      map[int64]data.Row
	NextRID        int64
	TombstoneCount int
	PKIndex        *btree.BPlusTree // primary key B+Tree index (nil if no PK)
	Indexes        map[string]*data.Index
	LastInsertID   int64
	Dirty          bool // tracks if table has unsaved changes
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

// LiveRows returns a safe copy of all active rows (excluding tombstones),
// sorted by PK for deterministic ordering.
func (t *Table) LiveRows() []data.Row {
	t.RLock()
	defer t.RUnlock()

	result := make([]data.Row, 0, len(t.RowsByRID))
	t.ForEachLiveRowUnsafe(func(row data.Row) bool {
		result = append(result, row.Copy())
		return true
	})
	return result
}

// ForEachLiveRowUnsafe calls fn for each non-deleted row in PK order.
// The row passed to fn is a direct reference — DO NOT mutate row.Values.
// If fn returns false, iteration stops early.
// MUST be called while holding at least a read lock.
func (t *Table) ForEachLiveRowUnsafe(fn func(row data.Row) bool) {
	if t == nil || t.RowsByRID == nil {
		return
	}

	if t.PKIndex != nil && t.PKIndex.Size() > 0 {
		rids := t.PKIndex.All()
		for _, rid := range rids {
			row, ok := t.RowsByRID[rid]
			if ok && !row.Deleted {
				if !fn(row) {
					return
				}
			}
		}
		return
	}

	for _, row := range t.RowsByRID {
		if !row.Deleted {
			if !fn(row) {
				return
			}
		}
	}
}

// LiveRowCount returns the number of active rows.
func (t *Table) LiveRowCount() int {
	t.RLock()
	defer t.RUnlock()
	count := 0
	if t.RowsByRID != nil {
		for _, row := range t.RowsByRID {
			if !row.Deleted {
				count++
			}
		}
	}
	return count
}

// RowByRID returns a specific row by its RID if it exists and is not deleted.
func (t *Table) RowByRID(rid int64) (data.Row, bool) {
	t.RLock()
	defer t.RUnlock()
	if t.RowsByRID == nil {
		return data.Row{}, false
	}
	row, found := t.RowsByRID[rid]
	if !found || row.Deleted {
		return data.Row{}, false
	}
	return row.Copy(), true
}

// VacuumUnsafe deletes tombstones from the map.
// IMPORTANT: Must be called while holding write lock!
func (t *Table) VacuumUnsafe() {
	if t.RowsByRID == nil || t.TombstoneCount == 0 {
		return
	}
	for rid, row := range t.RowsByRID {
		if row.Deleted {
			delete(t.RowsByRID, rid)
		}
	}
	t.TombstoneCount = 0
}

// Vacuum reclaims space by removing tombstoned rows from the map.
func (t *Table) Vacuum() {
	t.Lock()
	defer t.Unlock()
	t.VacuumUnsafe()
}

// Insert adds a new row to the table with full validation and auto-increment support
func (t *Table) Insert(mutRow data.Row, tx *transaction.Transaction) error {
	row := mutRow.Copy() // prevent mutation of caller's data

	// Acquire write lock for the entire operation
	t.Lock()
	defer t.Unlock()

	if tx != nil {
		slog.Debug("Insert operation", "table", t.Name, "tx_id", tx.TxID)
	}

	// 1. Handle auto-increment primary key FIRST (before validation)
	var autoIncCol *Column
	var autoIncColIdx int
	for i := range t.Schema.Columns {
		if t.Schema.Columns[i].AutoIncrement && t.Schema.Columns[i].PrimaryKey {
			autoIncCol = &t.Schema.Columns[i]
			autoIncColIdx = i
			break
		}
	}

	if autoIncCol != nil {
		// Generate next ID
		nextID := t.LastInsertID + 1

		// Allow user to override auto-increment
		if val := row.Values[autoIncColIdx]; val != nil {
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
		row.Values[autoIncColIdx] = nextID
		t.LastInsertID = nextID
	} else {
		// If PK is not auto-increment, it must be provided
		pkCol := t.Schema.GetPrimaryKeyColumn()
		if pkCol != nil {
			pkIdx := t.Schema.GetColumnIndex(pkCol.Name)
			if pkIdx == -1 || row.Values[pkIdx] == nil {
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
		colIdx := t.Schema.GetColumnIndex(colName)
		if colIdx == -1 {
			continue
		}
		val := row.Values[colIdx]
		if val == nil {
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

	// 4. Assign new RID
	if t.RowsByRID == nil {
		t.RowsByRID = make(map[int64]data.Row)
	}
	if t.NextRID == 0 {
		t.NextRID = 1
	}
	rid := t.NextRID
	t.NextRID++
	row.RID = rid
	row.Deleted = false

	// 5. Update PKIndex
	if t.PKIndex != nil {
		pkCol := t.Schema.GetPrimaryKeyColumn()
		if pkCol != nil {
			pkIdx := t.Schema.GetColumnIndex(pkCol.Name)
			if pkIdx != -1 && row.Values[pkIdx] != nil {
				pkVal := row.Values[pkIdx]
				if err := t.PKIndex.Insert(pkVal, rid); err != nil {
					return &errors.ConstraintError{
						Table:      t.Name,
						Column:     pkCol.Name,
						Value:      pkVal,
						Constraint: "primary_key",
						Reason:     "duplicate primary key (B+Tree)",
					}
				}
			}
		}
	}

	// 6. Update hash indexes
	for colName, idx := range t.Indexes {
		colIdx := t.Schema.GetColumnIndex(colName)
		if colIdx != -1 {
			val := row.Values[colIdx]
			if val != nil {
				idx.Data[val] = append(idx.Data[val], rid)
			}
		}
	}

	// 7. Store in map
	t.RowsByRID[rid] = row

	// 8. Mark dirty
	t.MarkDirtyUnsafe()
	return nil
}

// SelectAll returns all rows of the table.
// Order is not guaranteed. For sorted results, use an index.
func (t *Table) SelectAll(tx *transaction.Transaction) []data.Row {
	t.RLock()
	defer t.RUnlock()

	if tx != nil {
		slog.Debug("SelectAll operation", "table", t.Name, "tx_id", tx.TxID)
	}

	result := make([]data.Row, 0, len(t.RowsByRID))
	for _, row := range t.RowsByRID {
		if !row.Deleted {
			result = append(result, row)
		}
	}
	return result
}

// Select returns rows that match the given predicate
// Order is not guaranteed.
func (t *Table) Select(predicate func(data.Row) bool, tx *transaction.Transaction) []data.Row {
	t.RLock()
	defer t.RUnlock()

	if tx != nil {
		slog.Debug("Select operation", "table", t.Name, "tx_id", tx.TxID)
	}

	// Pre-allocate slice to capacity of RowsByRID to prevent reallocation overhead
	result := make([]data.Row, 0, len(t.RowsByRID))
	for _, row := range t.RowsByRID {
		if !row.Deleted && predicate(row) {
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

	if t.RowsByRID == nil {
		return data.Row{}, false
	}

	if tx != nil {
		slog.Debug("SelectByIndex operation", "table", t.Name, "column", colName, "tx_id", tx.TxID)
	}

	// Fast path: use B+Tree for PK lookups — O(log n) instead of hash lookup
	if t.PKIndex != nil {
		pkCol := t.Schema.GetPrimaryKeyColumn()
		if pkCol != nil && pkCol.Name == colName {
			rid, found := t.PKIndex.Search(value)
			if !found {
				return data.Row{}, false
			}
			row, ok := t.RowsByRID[rid]
			if !ok || row.Deleted {
				return data.Row{}, false
			}
			return row.Copy(), true
		}
	}

	// Fallback: use hash index for non-PK unique columns
	idx, exists := t.Indexes[colName]
	if !exists || !idx.Unique {
		return data.Row{}, false
	}

	// Convert value to int64 if it's an integer type for comparison
	if intVal, ok := value.(int); ok {
		value = int64(intVal)
	}

	rids, found := idx.Data[value]
	if !found || len(rids) == 0 {
		return data.Row{}, false
	}

	row, ok := t.RowsByRID[rids[0]]
	if !ok || row.Deleted {
		return data.Row{}, false
	}
	return row.Copy(), true
}

// Update modifies rows that match the given predicate
// Returns the number of rows updated
func (t *Table) Update(predicate func(data.Row) bool, updates map[string]interface{}, tx *transaction.Transaction) (int, error) {
	t.Lock()
	defer t.Unlock()

	if tx != nil {
		slog.Debug("Update operation", "table", t.Name, "tx_id", tx.TxID)
	}

	// 1. Collect matching RIDs first
	var matchedRIDs []int64
	for rid, row := range t.RowsByRID {
		if !row.Deleted && predicate(row) {
			matchedRIDs = append(matchedRIDs, rid)
		}
	}

	// 2. Apply updates
	count := 0
	pkCol := t.Schema.GetPrimaryKeyColumn()

	for _, rid := range matchedRIDs {
		row := t.RowsByRID[rid]
		newRow := row.Copy() // Deep copy before mutation

		// Phase 2a: Validation (check types and uniqueness before any mutations)
		for colName, newValue := range updates {
			colIdx := t.Schema.GetColumnIndex(colName)
			if colIdx == -1 {
				return 0, &errors.ColumnNotFoundError{
					TableName:  t.Name,
					ColumnName: colName,
				}
			}

			oldValue := newRow.Values[colIdx]
			if oldValue == newValue {
				continue
			}

			col := &t.Schema.Columns[colIdx]

			if err := t.validateType(col.Name, newValue, col.Type); err != nil {
				return 0, err
			}

			// Check PK uniqueness
			if pkCol != nil && pkCol.Name == colName {
				if t.PKIndex != nil {
					if foundRID, found := t.PKIndex.Search(newValue); found && foundRID != rid {
						return 0, &errors.ConstraintError{
							Table:      t.Name,
							Column:     colName,
							Value:      newValue,
							Constraint: "primary_key",
							Reason:     "duplicate primary key",
						}
					}
				}
			}

			// Check hash index uniqueness
			if idx, exists := t.Indexes[colName]; exists && idx.Unique {
				if rids, found := idx.Data[newValue]; found {
					for _, r := range rids {
						if r != rid {
							return 0, &errors.ConstraintError{
								Table:      t.Name,
								Column:     colName,
								Value:      newValue,
								Constraint: "unique",
								Reason:     "duplicate value in unique index",
							}
						}
					}
				}
			}
		}

		// Phase 2b: Mutation (safe to perform because all validations passed)
		for colName, newValue := range updates {
			colIdx := t.Schema.GetColumnIndex(colName)
			oldValue := newRow.Values[colIdx]
			if oldValue == newValue {
				continue
			}

			// Update PK index if PK column changed
			if pkCol != nil && pkCol.Name == colName {
				if t.PKIndex != nil {
					t.PKIndex.Delete(oldValue)
					_ = t.PKIndex.Insert(newValue, rid)
				}
			}

			// Update hash indexes
			if idx, exists := t.Indexes[colName]; exists {
				if oldList, found := idx.Data[oldValue]; found {
					var newList []int64
					for _, r := range oldList {
						if r != rid {
							newList = append(newList, r)
						}
					}
					if len(newList) == 0 {
						delete(idx.Data, oldValue)
					} else {
						idx.Data[oldValue] = newList
					}
				}
				idx.Data[newValue] = append(idx.Data[newValue], rid)
			}

			newRow.Values[colIdx] = newValue
		}

		t.RowsByRID[rid] = newRow
		count++
	}

	if count > 0 {
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
		slog.Debug("Delete operation", "table", t.Name, "tx_id", tx.TxID)
	}

	// 1. Collect matching RIDs
	var matchedRIDs []int64
	for rid, row := range t.RowsByRID {
		if !row.Deleted && predicate(row) {
			matchedRIDs = append(matchedRIDs, rid)
		}
	}

	// 2. Tombstone each matched row
	deleted := 0
	pkCol := t.Schema.GetPrimaryKeyColumn()

	for _, rid := range matchedRIDs {
		row := t.RowsByRID[rid]

		// Mark as deleted (tombstone)
		row.Deleted = true
		t.RowsByRID[rid] = row
		deleted++
		t.TombstoneCount++

		// Remove from PK index
		if pkCol != nil && t.PKIndex != nil {
			pkIdx := t.Schema.GetColumnIndex(pkCol.Name)
			if pkIdx != -1 && row.Values[pkIdx] != nil {
				_ = t.PKIndex.Delete(row.Values[pkIdx])
			}
		}

		// Remove from hash indexes
		for colName, idx := range t.Indexes {
			colIdx := t.Schema.GetColumnIndex(colName)
			if colIdx != -1 {
				val := row.Values[colIdx]
				if val != nil {
					if rids, found := idx.Data[val]; found {
						var newList []int64
						for _, r := range rids {
							if r != rid {
								newList = append(newList, r)
							}
						}
						if len(newList) == 0 {
							delete(idx.Data, val)
						} else {
							idx.Data[val] = newList
						}
					}
				}
			}
		}
	}

	if deleted > 0 {
		t.MarkDirtyUnsafe()
		// Auto-vacuum if tombstones exceed threshold
		if t.TombstoneCount > 10000 {
			t.VacuumUnsafe()
		}
	}

	return deleted, nil
}

// validateRow validates a row against the table schema
// Must be called while holding a lock
func (t *Table) validateRow(row data.Row) error {
	for i, col := range t.Schema.Columns {
		value := row.Values[i]

		// Check NOT NULL constraint
		if col.NotNull && value == nil {
			return &errors.ConstraintError{
				Table:      t.Name,
				Column:     col.Name,
				Constraint: "not_null",
				Reason:     "missing required value",
			}
		}

		// Skip type validation if value doesn't exist
		if value == nil {
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
	if value == nil {
		return nil
	}
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

// rebuildIndexesUnsafe rebuilds all indexes (hash indexes + PKIndex B+Tree)
// IMPORTANT: Must be called while holding write lock!
func (t *Table) rebuildIndexesUnsafe() {
	// Clear existing hash indexes
	for _, idx := range t.Indexes {
		idx.Data = make(map[interface{}][]int64)
	}

	// Rebuild PKIndex B+Tree if present
	if t.PKIndex != nil {
		t.PKIndex.Clear()
		pkCol := t.Schema.GetPrimaryKeyColumn()
		if pkCol != nil {
			pkIdx := t.Schema.GetColumnIndex(pkCol.Name)
			if pkIdx != -1 {
				for rid, row := range t.RowsByRID {
					if !row.Deleted && row.Values[pkIdx] != nil {
						t.PKIndex.Insert(row.Values[pkIdx], rid)
					}
				}
			}
		}
	}

	// Rebuild hash indexes from current rows
	for rid, row := range t.RowsByRID {
		if !row.Deleted {
			for colName, idx := range t.Indexes {
				colIdx := t.Schema.GetColumnIndex(colName)
				if colIdx != -1 {
					val := row.Values[colIdx]
					if val != nil {
						idx.Data[val] = append(idx.Data[val], rid)
					}
				}
			}
		}
	}
}

// normalizeToInt64 converts various numeric types to int64
// Returns the int64 value and true if successful, 0 and false otherwise
// InsertReplay appends a row during WAL recovery (no validation, no locking).
// This is safe because recovery runs single-threaded before the table is live.
func (t *Table) InsertReplay(row data.Row) {
	if t.RowsByRID == nil {
		t.RowsByRID = make(map[int64]data.Row)
	}

	// Ensure RID is set correctly and NextRID is advanced
	if row.RID == 0 {
		if t.NextRID == 0 {
			t.NextRID = 1
		}
		row.RID = t.NextRID
		t.NextRID++
	} else {
		if row.RID >= t.NextRID {
			t.NextRID = row.RID + 1
		}
	}

	rid := row.RID

	t.RowsByRID[rid] = row

	if t.PKIndex != nil {
		pkCol := t.Schema.GetPrimaryKeyColumn()
		if pkCol != nil {
			pkIdx := t.Schema.GetColumnIndex(pkCol.Name)
			if pkIdx != -1 && row.Values[pkIdx] != nil {
				t.PKIndex.Insert(row.Values[pkIdx], rid)
			}
		}
	}
	// Update hash indexes too
	for colName, idx := range t.Indexes {
		colIdx := t.Schema.GetColumnIndex(colName)
		if colIdx != -1 {
			val := row.Values[colIdx]
			if val != nil {
				idx.Data[val] = append(idx.Data[val], rid)
			}
		}
	}
}

// InsertReplayRID adds a row during WAL recovery with an explicit RID
func (t *Table) InsertReplayRID(row data.Row) {
	t.InsertReplay(row)
}

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

// GetPrimaryKeyValue extracts the primary key value from a row as a string
// This is used for WAL record keys which require a string key
func (t *Table) GetPrimaryKeyValue(row data.Row) (string, error) {
	pkCol := t.Schema.GetPrimaryKeyColumn()
	if pkCol == nil {
		return "", fmt.Errorf("table %s has no primary key", t.Name)
	}

	pkIdx := t.Schema.GetColumnIndex(pkCol.Name)
	if pkIdx == -1 || row.Values[pkIdx] == nil {
		return "", fmt.Errorf("row missing primary key column %s", pkCol.Name)
	}
	val := row.Values[pkIdx]

	// Convert value to string based on type
	switch v := val.(type) {
	case string:
		return v, nil
	case int64:
		return fmt.Sprintf("%d", v), nil
	case int:
		return fmt.Sprintf("%d", v), nil
	case float64:
		// Check if it's a whole number (common when loaded from JSON)
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), nil
		}
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

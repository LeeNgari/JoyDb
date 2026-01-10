package operations

import (

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/errors"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/query/validation"
)

// Insert adds a new row to the table with full validation and auto-increment support
func Insert(table *schema.Table, mutRow data.Row) error {
	row := mutRow.Copy() // prevent mutation of caller's data

	// Acquire write lock for the entire operation
	table.Lock()
	defer table.Unlock()

	// 1. Handle auto-increment primary key FIRST (before validation)
	var autoIncCol *schema.Column
	for _, col := range table.Schema.Columns {
		if col.AutoIncrement && col.PrimaryKey {
			autoIncCol = &col
			break
		}
	}

	if autoIncCol != nil {
		// Generate next ID
		nextID := table.LastInsertID + 1

		// Allow user to override auto-increment (for imports, migrations, etc.)
		if val, exists := row[autoIncCol.Name]; exists {
			userID, ok := normalizeToInt64(val)
			if !ok {
				return &errors.ConstraintError{
					Table:      table.Name,
					Column:     autoIncCol.Name,
					Value:      val,
					Constraint: "type_mismatch",
					Reason:     "auto-increment column must be integer",
				}
			}
			// Prevent sequence conflicts
			if userID <= table.LastInsertID {
				return &errors.ConstraintError{
					Table:      table.Name,
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
		table.LastInsertID = nextID
	} else {
		// If PK is not auto-increment, it must be provided
		pkCol := table.Schema.GetPrimaryKeyColumn()
		if pkCol != nil {
			if _, exists := row[pkCol.Name]; !exists {
				return &errors.ConstraintError{
					Table:      table.Name,
					Column:     pkCol.Name,
					Constraint: "primary_key",
					Reason:     "primary key value required",
				}
			}
		}
	}

	// 2. Validate the row (types, NOT NULL, etc.)
	// Row now has auto-increment value set, so validation will work
	if err := validation.ValidateRow(table, row, -1); err != nil {
		return err
	}

	// 3. Check unique/primary constraints using current indexes
	for colName, idx := range table.Indexes {
		val, exists := row[colName]
		if !exists {
			continue
		}

		if idx.Unique {
			if _, found := idx.Data[val]; found {
				return &errors.ConstraintError{
					Table:      table.Name,
					Column:     colName,
					Value:      val,
					Constraint: "unique",
					Reason:     "duplicate value",
				}
			}
		}
	}

	// 4. Get new position (BEFORE append)
	newRowPos := len(table.Rows)

	// 5. Everything passed → safe to append
	table.Rows = append(table.Rows, row)

	// 6. Update all indexes
	for colName, idx := range table.Indexes {
		if val, exists := row[colName]; exists {
			idx.Data[val] = append(idx.Data[val], newRowPos)
		}
	}

	// 7. Mark table as dirty (has unsaved changes)
	table.MarkDirtyUnsafe()

	return nil
}

// normalizeToInt64 converts various numeric types to int64
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

package executor

import (
	"fmt"

	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/domain/errors"
)

// validateInsertFKs verifies that inserting/updating a row in the child table
// respects all defined foreign key constraints (referential integrity).
func validateInsertFKs(tableName string, row data.Row, ctx *ExecutionContext) error {
	table, ok := ctx.Database.Tables[tableName]
	if !ok {
		return nil
	}

	if table.Schema == nil || len(table.Schema.ForeignKeys) == 0 {
		return nil
	}

	for _, fk := range table.Schema.ForeignKeys {
		idx := table.Schema.GetColumnIndex(fk.ColumnName)
		if idx == -1 { continue }
		val := row.Values[idx]
		if val == nil {
			// NULL foreign keys are typically allowed in SQL (unless NOT NULL is set, which is checked by validateRow)
			continue
		}

		// Find parent table
		parentTable, ok := ctx.Database.Tables[fk.RefTableName]
		if !ok {
			return fmt.Errorf("foreign key constraint error: referenced table %q does not exist", fk.RefTableName)
		}

		// Look up the value in the parent table
		found := false
		// Try using SelectByIndex first (fast path) - it manages its own locks
		if _, ok := parentTable.SelectByIndex(fk.RefColumnName, val, ctx.Transaction); ok {
			found = true
		} else {
			// Slow path fallback: search all rows
			parentTable.RLock()
			pIdx := parentTable.Schema.GetColumnIndex(fk.RefColumnName)
			parentTable.ForEachLiveRowUnsafe(func(pRow data.Row) bool {
				if pIdx == -1 { return true }
				pVal := pRow.Values[pIdx]
				if pVal == val {
					found = true
					return false
				}
				// Cross-numeric type checking (e.g. int vs int64)
				if i1, ok1 := normalizeNumeric(pVal); ok1 {
					if i2, ok2 := normalizeNumeric(val); ok2 {
						if i1 == i2 {
							found = true
							return false
						}
					}
				}
				return true
			})
			parentTable.RUnlock()
		}

		if !found {
			return &errors.ConstraintError{
				Table:      tableName,
				Column:     fk.ColumnName,
				Value:      val,
				Constraint: "foreign_key",
				Reason:     fmt.Sprintf("referenced value %v in %s.%s does not exist", val, fk.RefTableName, fk.RefColumnName),
			}
		}
	}

	return nil
}

// validateDeleteFKs verifies that deleting/updating a parent row does not
// violate referential integrity constraints in child tables.
func validateDeleteFKs(tableName string, oldRow data.Row, ctx *ExecutionContext) error {
	// Look at all tables in the database to see if any have a foreign key referencing us!
	for _, childTable := range ctx.Database.Tables {
		if childTable.Schema == nil {
			continue
		}
		for _, fk := range childTable.Schema.ForeignKeys {
			if fk.RefTableName != tableName {
				continue
			}

			// This childTable has a foreign key referencing tableName!
			// We must ensure no row in childTable references oldRow's value for RefColumnName.
			table, _ := ctx.Database.Tables[tableName]
			pIdx := table.Schema.GetColumnIndex(fk.RefColumnName)
			if pIdx == -1 { continue }
			parentVal := oldRow.Values[pIdx]
			if parentVal == nil {
				continue
			}

			childTable.RLock()
			conflict := false
			cIdx := childTable.Schema.GetColumnIndex(fk.ColumnName)
			// Check if any child row references this value
			childTable.ForEachLiveRowUnsafe(func(cRow data.Row) bool {
				if cIdx == -1 { return true }
				childVal := cRow.Values[cIdx]
				if childVal == nil {
					return true
				}

				if childVal == parentVal {
					conflict = true
					return false
				}

				// Cross-numeric checking
				if i1, ok1 := normalizeNumeric(childVal); ok1 {
					if i2, ok2 := normalizeNumeric(parentVal); ok2 {
						if i1 == i2 {
							conflict = true
							return false
						}
					}
				}
				return true
			})
			childTable.RUnlock()

			if conflict {
				return &errors.ConstraintError{
					Table:      tableName,
					Column:     fk.RefColumnName,
					Value:      parentVal,
					Constraint: "foreign_key",
					Reason:     fmt.Sprintf("cannot delete/update row: value is referenced by child table %q", childTable.Name),
				}
			}
		}
	}
	return nil
}

// normalizeNumeric converts float64, int64, int, etc. to int64 for cross-type comparison
func normalizeNumeric(val interface{}) (int64, bool) {
	switch v := val.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
	}
	return 0, false
}

package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/errors"
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
		val, exists := row.Data[fk.ColumnName]
		if !exists || val == nil {
			// NULL foreign keys are typically allowed in SQL (unless NOT NULL is set, which is checked by validateRow)
			continue
		}

		// Find parent table
		parentTable, ok := ctx.Database.Tables[fk.RefTableName]
		if !ok {
			return fmt.Errorf("foreign key constraint error: referenced table %q does not exist", fk.RefTableName)
		}

		// Look up the value in the parent table
		parentTable.RLock()
		found := false
		// Try using SelectByIndex first (fast path)
		if _, ok := parentTable.SelectByIndex(fk.RefColumnName, val, ctx.Transaction); ok {
			found = true
		} else {
			// Slow path fallback: search all rows
			for _, pRow := range parentTable.Rows {
				pVal := pRow.Data[fk.RefColumnName]
				if pVal == val {
					found = true
					break
				}
				// Cross-numeric type checking (e.g. int vs int64)
				if i1, ok1 := normalizeNumeric(pVal); ok1 {
					if i2, ok2 := normalizeNumeric(val); ok2 {
						if i1 == i2 {
							found = true
							break
						}
					}
				}
			}
		}
		parentTable.RUnlock()

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
			parentVal, exists := oldRow.Data[fk.RefColumnName]
			if !exists || parentVal == nil {
				continue
			}

			childTable.RLock()
			conflict := false
			// Check if any child row references this value
			for _, cRow := range childTable.Rows {
				childVal, cExists := cRow.Data[fk.ColumnName]
				if !cExists || childVal == nil {
					continue
				}

				if childVal == parentVal {
					conflict = true
					break
				}

				// Cross-numeric checking
				if i1, ok1 := normalizeNumeric(childVal); ok1 {
					if i2, ok2 := normalizeNumeric(parentVal); ok2 {
						if i1 == i2 {
							conflict = true
							break
						}
					}
				}
			}
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

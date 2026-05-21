package executor

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// executeUpdateNode handles UPDATE using tree-walking pattern
func executeUpdateNode(node *plan.UpdateNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	table, ok := ctx.Database.Tables[node.TableName]
	if !ok {
		return nil, newTableNotFoundError(node.TableName)
	}

	// Validate foreign keys for all affected rows BEFORE any mutations or logging
	table.RLock()
	var rowsToValidate []struct{ oldRow, newRow data.Row }
	for _, row := range table.Rows {
		if node.Predicate(row) {
			// Compute newRow
			newRow := row.Copy()
			for col, val := range node.Updates.Data {
				newRow.Data[col] = val
			}
			rowsToValidate = append(rowsToValidate, struct{ oldRow, newRow data.Row }{row.Copy(), newRow})
		}
	}
	table.RUnlock()

	for _, item := range rowsToValidate {
		// 1. Check if this is a child table and we are inserting/updating referencing column value
		if err := validateInsertFKs(node.TableName, item.newRow, ctx); err != nil {
			return nil, err
		}

		// 2. Check if this is a parent table and we are modifying a referenced column value
		hasKeyChange := false
		for _, childTable := range ctx.Database.Tables {
			if childTable.Schema == nil {
				continue
			}
			for _, fk := range childTable.Schema.ForeignKeys {
				if fk.RefTableName == node.TableName {
					oldVal := item.oldRow.Data[fk.RefColumnName]
					newVal := item.newRow.Data[fk.RefColumnName]
					if oldVal != newVal {
						hasKeyChange = true
						break
					}
				}
			}
			if hasKeyChange {
				break
			}
		}
		if hasKeyChange {
			if err := validateDeleteFKs(node.TableName, item.oldRow, ctx); err != nil {
				return nil, err
			}
		}
	}

	// If WAL is enabled, capture old rows and their keys before update
	type oldRowInfo struct {
		key    string
		oldRow data.Row
	}
	var oldRowInfos []oldRowInfo

	if ctx.WALManager != nil {
		table.RLock()
		for _, row := range table.Rows {
			if node.Predicate(row) {
				key, keyErr := table.GetPrimaryKeyValue(row)
				if keyErr == nil {
					oldRowInfos = append(oldRowInfos, oldRowInfo{
						key:    key,
						oldRow: row.Copy(),
					})
				}
			}
		}
		table.RUnlock()
	}

	// Log to WAL before successful update
	if ctx.WALManager != nil && len(oldRowInfos) > 0 {
		for _, info := range oldRowInfos {
			// Compute the new row by applying updates to old row
			newRow := info.oldRow.Copy()
			for col, val := range node.Updates.Data {
				newRow.Data[col] = val
			}

			if err := ctx.WALManager.LogUpdate(ctx.Transaction, table, info.key, info.oldRow, newRow); err != nil {
				return nil, err
			}
		}
	}

	// Use domain model to update
	rowsAffected, err := table.Update(node.Predicate, node.Updates, ctx.Transaction)
	if err != nil {
		// If table update fails, the engine will call WALManager.Abort(tx)
		return nil, err
	}

	return &IntermediateResult{
		Rows:   []data.Row{},
		Schema: nil,
		Metadata: map[string]interface{}{
			"operation":     "UPDATE",
			"rows_affected": rowsAffected,
		},
	}, nil
}

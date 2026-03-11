package executor

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// executeDeleteNode handles DELETE using tree-walking pattern
func executeDeleteNode(node *plan.DeleteNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	table, ok := ctx.Database.Tables[node.TableName]
	if !ok {
		return nil, newTableNotFoundError(node.TableName)
	}

	// If WAL is enabled, we need to capture old rows before delete
	var oldRows []data.Row
	if ctx.WALManager != nil {
		table.RLock()
		for _, row := range table.Rows {
			if node.Predicate(row) {
				oldRows = append(oldRows, row.Copy())
			}
		}
		table.RUnlock()
	}

	// Log to WAL before successful delete (WAL-first execution)
	if ctx.WALManager != nil && len(oldRows) > 0 {
		for _, oldRow := range oldRows {
			key, keyErr := table.GetPrimaryKeyValue(oldRow)
			if keyErr == nil {
				if err := ctx.WALManager.LogDelete(ctx.Transaction, table, key, oldRow); err != nil {
					return nil, err
				}
			}
		}
	}

	// Use domain model to delete
	rowsAffected, err := table.Delete(node.Predicate, ctx.Transaction)
	if err != nil {
		// If table delete fails, the engine will call WALManager.Abort(tx)
		return nil, err
	}

	return &IntermediateResult{
		Rows:   []data.Row{},
		Schema: nil,
		Metadata: map[string]interface{}{
			"operation":     "DELETE",
			"rows_affected": rowsAffected,
		},
	}, nil
}

package executor

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// executeInsertNode handles INSERT using tree-walking pattern
func executeInsertNode(node *plan.InsertNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	table, ok := ctx.Database.Tables[node.TableName]
	if !ok {
		return nil, newTableNotFoundError(node.TableName)
	}

	// Log to WAL before successful insert
	if ctx.WALManager != nil {
		if err := ctx.WALManager.LogInsert(ctx.Transaction, table, node.Row); err != nil {
			return nil, err
		}
	}

	// Insert the row using domain model
	if err := table.Insert(node.Row, ctx.Transaction); err != nil {
		// If table insert fails, the engine will call WALManager.Abort(tx)
		return nil, err
	}

	return &IntermediateResult{
		Rows:   []data.Row{},
		Schema: nil,
		Metadata: map[string]interface{}{
			"operation":     "INSERT",
			"rows_affected": 1,
		},
	}, nil
}

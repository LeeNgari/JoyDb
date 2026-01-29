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

	// Insert the row using domain model
	if err := table.Insert(node.Row, ctx.Transaction); err != nil {
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

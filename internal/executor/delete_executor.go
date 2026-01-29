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

	// Use domain model to delete
	rowsAffected, err := table.Delete(node.Predicate, ctx.Transaction)
	if err != nil {
		return nil, err
	}

	return &IntermediateResult{
		Rows: []data.Row{},
		Metadata: map[string]interface{}{
			"operation":     "DELETE",
			"rows_affected": rowsAffected,
		},
	}, nil
}

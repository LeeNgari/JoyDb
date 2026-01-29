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

	// Use domain model to update
	rowsAffected, err := table.Update(node.Predicate, node.Updates, ctx.Transaction)
	if err != nil {
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

package executor

import (
	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/plan"
)

func executeScan(node *plan.ScanNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	table, ok := ctx.Database.Tables[node.TableName]
	if !ok {
		return nil, newTableNotFoundError(node.TableName)
	}

	var rows []data.Row
	if node.Predicate == nil {
		rows = table.SelectAll(ctx.Transaction)
	} else {
		rows = table.Select(node.Predicate, ctx.Transaction)
	}

	return &IntermediateResult{
		Rows:   rows,
		Schema: table.Schema,
		Metadata: map[string]interface{}{
			"table":     node.TableName,
			"scan_type": "sequential",
			"row_count": len(rows),
		},
	}, nil
}

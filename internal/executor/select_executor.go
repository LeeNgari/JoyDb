package executor

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

// executeSelectNode handles SelectNode using tree-walking pattern
// Returns IntermediateResult for composition with other nodes
func executeSelectNode(node *plan.SelectNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	var rows []data.Row

	if len(node.Children()) > 0 {
		// Execute child (JOIN tree or other operation) recursively
		childResult, err := executeNode(node.Children()[0], ctx)
		if err != nil {
			return nil, err
		}
		rows = childResult.Rows
	} else {
		// No children - simple table scan
		scanNode := &plan.ScanNode{
			TableName:   node.TableName,
			Predicate:   node.Predicate,
			Transaction: node.Transaction,
		}
		scanResult, err := executeScan(scanNode, ctx)
		if err != nil {
			return nil, err
		}
		rows = scanResult.Rows
	}

	// Apply projection
	projectedRows := make([]data.Row, len(rows))
	for i, row := range rows {
		projectedRows[i] = projection.ProjectRow(row, node.Projection, node.TableName)
	}

	return &IntermediateResult{
		Rows: projectedRows,
		Metadata: map[string]interface{}{
			"projection": node.Projection,
			"row_count":  len(projectedRows),
		},
	}, nil
}

package executor

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

// executeSelectNode handles SelectNode using tree-walking pattern
// Returns IntermediateResult for composition with other nodes
func executeSelectNode(node *plan.SelectNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	var rows []data.Row

	var resultSchema *schema.TableSchema
	if len(node.Children()) > 0 {
		// Execute child (JOIN tree or other operation) recursively
		childResult, err := executeNode(node.Children()[0], ctx)
		if err != nil {
			return nil, err
		}
		rows = childResult.Rows
		resultSchema = childResult.Schema
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
		resultSchema = scanResult.Schema
	}

	// Apply predicate
	if node.Predicate != nil {
		filteredRows := make([]data.Row, 0)
		for _, row := range rows {
			if node.Predicate(row) {
				filteredRows = append(filteredRows, row)
			}
		}
		rows = filteredRows
	}

	// Apply projection
	projectedRows := make([]data.Row, len(rows))
	for i, row := range rows {
		// Convert Row to JoinedRow for projector (ProjectJoinedRow handles qualified names)
		joined := data.JoinedRow{Data: row.Data}
		projectedJoined := projection.ProjectJoinedRow(joined, node.Projection)
		projectedRows[i] = data.Row{Data: projectedJoined.Data}
	}

	return &IntermediateResult{
		Rows:   projectedRows,
		Schema: resultSchema,
		Metadata: map[string]interface{}{
			"projection": node.Projection,
			"row_count":  len(projectedRows),
			"filtered":   node.Predicate != nil,
		},
	}, nil
}

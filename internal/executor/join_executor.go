package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/query/operations/join"
)

// executeJoinNode recursively executes JOIN using tree-walking pattern
// This enables multi-way JOINs by recursively executing left/right children
func executeJoinNode(node *plan.JoinNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	// Recursively execute left child
	leftResult, err := executeNode(node.Left(), ctx)
	if err != nil {
		return nil, fmt.Errorf("left child execution failed: %w", err)
	}

	// Recursively execute right child
	rightResult, err := executeNode(node.Right(), ctx)
	if err != nil {
		return nil, fmt.Errorf("right child execution failed: %w", err)
	}

	// Get table names from metadata (for qualified column names)
	leftTableName := extractTableName(node.Left())
	rightTableName := extractTableName(node.Right())

	// Create temporary in-memory tables from child results
	leftTable := createTempTable(leftTableName, leftResult.Rows)
	rightTable := createTempTable(rightTableName, rightResult.Rows)

	// Execute JOIN using existing join operations
	joinedRows, err := join.ExecuteJoin(
		leftTable,
		rightTable,
		node.LeftOnCol,
		node.RightOnCol,
		node.JoinType,
		nil,  // No additional predicate at this level
		nil,  // No projection at this level
		ctx.Transaction,
	)
	if err != nil {
		return nil, fmt.Errorf("JOIN execution failed: %w", err)
	}

	// Convert JoinedRow back to Row
	rows := make([]data.Row, len(joinedRows))
	for i, jr := range joinedRows {
		rows[i] = data.NewRow(jr.Data)
	}

	return &IntermediateResult{
		Rows: rows,
		Metadata: map[string]interface{}{
			"join_type":   node.JoinType,
			"left_rows":   len(leftResult.Rows),
			"right_rows":  len(rightResult.Rows),
			"result_rows": len(rows),
		},
	}, nil
}

// extractTableName extracts table name from a plan node
func extractTableName(node plan.Node) string {
	switch n := node.(type) {
	case *plan.ScanNode:
		return n.TableName
	case *plan.SelectNode:
		return n.TableName
	default:
		// For other node types, use the node type as a placeholder
		return fmt.Sprintf("temp_%s", node.NodeType())
	}
}

// createTempTable creates an in-memory table from rows for JOIN operations
func createTempTable(tableName string, rows []data.Row) *schema.Table {
	if len(rows) == 0 {
		return &schema.Table{
			Name: tableName,
			Rows: []data.Row{},
			Schema: &schema.TableSchema{
				Columns: []schema.Column{},
			},
		}
	}

	// Infer schema from first row
	var columns []schema.Column
	for colName := range rows[0].Data {
		columns = append(columns, schema.Column{
			Name: colName,
			Type: schema.ColumnTypeText, // Generic type for temp tables
		})
	}

	return &schema.Table{
		Name: tableName,
		Rows: rows,
		Schema: &schema.TableSchema{
			Columns: columns,
		},
	}
}

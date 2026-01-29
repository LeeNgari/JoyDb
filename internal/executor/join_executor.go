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

	// Create temporary in-memory tables from child results using the propagated schema
	leftTable := createTempTable(leftTableName, leftResult.Rows, leftResult.Schema)
	rightTable := createTempTable(rightTableName, rightResult.Rows, rightResult.Schema)

	// Execute JOIN using existing join operations
	joinedRows, err := join.ExecuteJoin(
		leftTable,
		rightTable,
		node.LeftOnCol,
		node.RightOnCol,
		node.JoinType,
		nil, // No additional predicate at this level
		nil, // No projection at this level
		ctx.Transaction,
	)
	if err != nil {
		return nil, fmt.Errorf("JOIN execution failed: %w", err)
	}

	// Build the schema for the joined result (qualified names)
	joinedSchema := &schema.TableSchema{
		Columns: make([]schema.Column, 0, len(leftTable.Schema.Columns)+len(rightTable.Schema.Columns)),
	}
	for _, col := range leftTable.Schema.Columns {
		joinedSchema.Columns = append(joinedSchema.Columns, schema.Column{
			Name: fmt.Sprintf("%s.%s", leftTableName, col.Name),
			Type: col.Type,
		})
	}
	for _, col := range rightTable.Schema.Columns {
		joinedSchema.Columns = append(joinedSchema.Columns, schema.Column{
			Name: fmt.Sprintf("%s.%s", rightTableName, col.Name),
			Type: col.Type,
		})
	}

	// Convert JoinedRow back to Row
	rows := make([]data.Row, len(joinedRows))
	for i, jr := range joinedRows {
		rows[i] = data.NewRow(jr.Data)
	}

	return &IntermediateResult{
		Rows:   rows,
		Schema: joinedSchema,
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
	case *plan.JoinNode:
		// Recursive join names can be complex, use a placeholder
		return fmt.Sprintf("join_%p", n)
	default:
		return "temp_table"
	}
}

// createTempTable creates an in-memory table from rows and explicit schema
func createTempTable(tableName string, rows []data.Row, tableSchema *schema.TableSchema) *schema.Table {
	if tableSchema == nil {
		// Fallback for nodes that don't provide schema (should be fixed in all nodes)
		tableSchema = &schema.TableSchema{Columns: []schema.Column{}}
		if len(rows) > 0 {
			for colName := range rows[0].Data {
				tableSchema.Columns = append(tableSchema.Columns, schema.Column{
					Name: colName,
					Type: schema.ColumnTypeText,
				})
			}
		}
	}

	return &schema.Table{
		Name:   tableName,
		Rows:   rows,
		Schema: tableSchema,
	}
}

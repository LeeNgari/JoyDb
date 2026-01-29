package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// ColumnMetadata provides rich information about a result column
type ColumnMetadata struct {
	Name string // Column name
	Type string // Data type as string
}

// Result represents the outcome of executing a SQL statement
type Result struct {
	Columns      []string         // Column names
	Metadata     []ColumnMetadata // Column metadata
	Rows         []data.Row       // Result rows
	Message      string           // Status message
	RowsAffected int              // Rows affected by INSERT/UPDATE/DELETE
	Error        string           // Error message if any
}

// IntermediateResult represents results from node execution
// Used for composing results from child nodes during tree walking
type IntermediateResult struct {
	Rows     []data.Row             // Result rows
	Schema   *schema.TableSchema    // Schema of the result (for join planning)
	Metadata map[string]interface{} // Execution metadata
}

// newTableNotFoundError creates a consistent error for missing tables
func newTableNotFoundError(tableName string) error {
	return fmt.Errorf("table not found: %s", tableName)
}

// Execute is the main entry point for executing execution plans
// It dispatches to the appropriate executor based on node type using tree walking
func Execute(node plan.Node, db *schema.Database, tx *transaction.Transaction) (*Result, error) {
	ctx := &ExecutionContext{
		Database:    db,
		Transaction: tx,
		Config:      DefaultExecutionConfig(),
	}

	// Execute the plan tree recursively
	intermediate, err := executeNode(node, ctx)
	if err != nil {
		return nil, err
	}

	// Format the final result based on node type
	switch n := node.(type) {
	case *plan.SelectNode:
		return formatSelectResult(n, intermediate, db), nil
	case *plan.InsertNode:
		return formatInsertResult(intermediate), nil
	case *plan.UpdateNode:
		return formatUpdateResult(intermediate), nil
	case *plan.DeleteNode:
		return formatDeleteResult(intermediate), nil
	default:
		return nil, fmt.Errorf("unsupported plan node type: %T", node)
	}
}

// executeNode recursively executes a plan node and its children
// This is the core of the tree-walking executor
func executeNode(node plan.Node, ctx *ExecutionContext) (*IntermediateResult, error) {
	if node == nil {
		return &IntermediateResult{
			Rows:     []data.Row{},
			Metadata: map[string]interface{}{},
		}, nil
	}

	switch n := node.(type) {
	case *plan.ScanNode:
		return executeScan(n, ctx)
	case *plan.JoinNode:
		return executeJoinNode(n, ctx)
	case *plan.SelectNode:
		return executeSelectNode(n, ctx)
	case *plan.InsertNode:
		return executeInsertNode(n, ctx)
	case *plan.UpdateNode:
		return executeUpdateNode(n, ctx)
	case *plan.DeleteNode:
		return executeDeleteNode(n, ctx)
	default:
		return nil, fmt.Errorf("unsupported plan node type: %T", node)
	}
}

// findColumnInSchema finds a column by name in the table schema
func findColumnInSchema(table *schema.Table, colName string) *schema.Column {
	for i := range table.Schema.Columns {
		if table.Schema.Columns[i].Name == colName {
			return &table.Schema.Columns[i]
		}
	}
	return nil
}

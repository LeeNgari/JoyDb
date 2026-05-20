package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

type ColumnMetadata struct {
	Name string
	Type string
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
	return ExecuteWithWAL(node, db, tx, nil)
}

// ExecuteWithWAL executes a plan with optional WAL logging
func ExecuteWithWAL(node plan.Node, db *schema.Database, tx *transaction.Transaction, walMgr *manager.WALManager) (*Result, error) {
	ctx := &ExecutionContext{
		Database:    db,
		Transaction: tx,
		Config:      DefaultExecutionConfig(),
		WALManager:  walMgr,
	}

	intermediate, err := executeNode(node, ctx)
	if err != nil {
		return nil, err
	}

	switch n := node.(type) {
	case *plan.SelectNode:
		return formatSelectResult(n, intermediate, db), nil
	case *plan.InsertNode:
		return formatInsertResult(intermediate), nil
	case *plan.UpdateNode:
		return formatUpdateResult(intermediate), nil
	case *plan.DeleteNode:
		return formatDeleteResult(intermediate), nil
	case *plan.CreateTableNode:
		return formatDDLResult(intermediate), nil
	case *plan.DropTableNode:
		return formatDDLResult(intermediate), nil
	default:
		return nil, fmt.Errorf("unsupported plan node type: %T", node)
	}
}

// executeNode recursively executes a plan node and its children
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
	case *plan.CreateTableNode:
		return executeCreateTable(n, ctx)
	case *plan.DropTableNode:
		return executeDropTable(n, ctx)
	default:
		return nil, fmt.Errorf("unsupported plan node type: %T", node)
	}
}

// formatDDLResult formats the result for a DDL operation
func formatDDLResult(intermediate *IntermediateResult) *Result {
	msg := ""
	if intermediate.Metadata != nil {
		if m, ok := intermediate.Metadata["message"].(string); ok {
			msg = m
		}
	}
	return &Result{
		Message: msg,
	}
}

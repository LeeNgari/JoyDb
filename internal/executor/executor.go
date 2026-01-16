package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
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

// Execute is the main entry point for executing execution plans
// It dispatches to the appropriate executor based on node type
func Execute(node plan.Node, db *schema.Database) (*Result, error) {
	switch n := node.(type) {
	case *plan.SelectNode:
		return executeSelect(n, db)
	case *plan.InsertNode:
		return executeInsert(n, db)
	case *plan.UpdateNode:
		return executeUpdate(n, db)
	case *plan.DeleteNode:
		return executeDelete(n, db)
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

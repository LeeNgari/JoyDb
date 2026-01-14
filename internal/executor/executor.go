package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
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
}

// Execute is the main entry point for executing SQL statements
// It dispatches to the appropriate executor based on statement type
func Execute(stmt ast.Statement, db *schema.Database) (*Result, error) {
	switch s := stmt.(type) {
	case *ast.SelectStatement:
		return executeSelect(s, db)
	case *ast.InsertStatement:
		return executeInsert(s, db)
	case *ast.UpdateStatement:
		return executeUpdate(s, db)
	case *ast.DeleteStatement:
		return executeDelete(s, db)
	default:
		return nil, fmt.Errorf("unsupported statement type: %T", stmt)
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

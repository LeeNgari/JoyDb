package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/executor/predicate"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
)

// executeDelete handles DELETE statements
func executeDelete(stmt *ast.DeleteStatement, db *schema.Database) (*Result, error) {
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	// Build predicate from WHERE clause
	var pred func(data.Row) bool
	if stmt.Where != nil {
		var err error
		pred, err = predicate.Build(stmt.Where)
		if err != nil {
			return nil, err
		}
	} else {
		// No WHERE clause = delete all rows
		pred = func(data.Row) bool { return true }
	}

	// Use domain model to delete
	rowsAffected, err := table.Delete(pred)
	if err != nil {
		return nil, err
	}

	return &Result{
		Message:      fmt.Sprintf("DELETE %d", rowsAffected),
		RowsAffected: rowsAffected,
	}, nil
}

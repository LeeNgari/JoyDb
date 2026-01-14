package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/executor/predicate"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/util/types"
)

// executeUpdate handles UPDATE statements
func executeUpdate(stmt *ast.UpdateStatement, db *schema.Database) (*Result, error) {
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	// Build updates map
	updates := make(data.Row)
	for colName, valueExpr := range stmt.Updates {
		lit, ok := valueExpr.(*ast.Literal)
		if !ok {
			return nil, fmt.Errorf("only literals supported in SET clause")
		}

		// Get schema column for type conversion
		schemaCol := findColumnInSchema(table, colName)
		if schemaCol != nil {
			convertedLit, err := types.ConvertLiteralToSchemaType(lit, schemaCol.Type)
			if err != nil {
				return nil, fmt.Errorf("column '%s': %w", colName, err)
			}
			updates[colName] = convertedLit.Value
		} else {
			updates[colName] = lit.Value
		}
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
		// No WHERE clause = update all rows
		pred = func(data.Row) bool { return true }
	}

	// Use domain model to update
	rowsAffected, err := table.Update(pred, updates)
	if err != nil {
		return nil, err
	}

	return &Result{
		Message:      fmt.Sprintf("UPDATE %d", rowsAffected),
		RowsAffected: rowsAffected,
	}, nil
}

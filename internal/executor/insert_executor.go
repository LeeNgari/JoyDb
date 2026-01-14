package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/util/types"
)

// executeInsert handles INSERT statements
// Maps INSERT INTO table (cols) VALUES (vals) to crud.Insert
func executeInsert(stmt *ast.InsertStatement, db *schema.Database) (*Result, error) {
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	if len(stmt.Columns) != len(stmt.Values) {
		return nil, fmt.Errorf("column count (%d) does not match value count (%d)", len(stmt.Columns), len(stmt.Values))
	}

	// Build row from values with implicit type conversion
	row := make(data.Row)
	for i, col := range stmt.Columns {
		lit, ok := stmt.Values[i].(*ast.Literal)
		if !ok {
			return nil, fmt.Errorf("only literals supported in VALUES")
		}

		// Get schema column
		schemaCol := findColumnInSchema(table, col.Value)
		if schemaCol != nil {
			// Convert literal to match schema type (implicit type detection)
			convertedLit, err := types.ConvertLiteralToSchemaType(lit, schemaCol.Type)
			if err != nil {
				return nil, fmt.Errorf("column '%s': %w", col.Value, err)
			}
			row[col.Value] = convertedLit.Value
		} else {
			// Column not in schema, use value as-is
			row[col.Value] = lit.Value
		}
	}

	// Insert the row using domain model
	if err := table.Insert(row); err != nil {
		return nil, err
	}

	return &Result{
		Message: "INSERT 1",
	}, nil
}

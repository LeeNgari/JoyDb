package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
	"github.com/leengari/mini-rdbms/internal/executor/predicate"
)

// executeSelect handles SELECT statements without JOINs
// For SELECT with JOINs, see join_executor.go
func executeSelect(stmt *ast.SelectStatement, db *schema.Database) (*Result, error) {
	// If there are JOINs, use the JOIN executor
	if len(stmt.Joins) > 0 {
		return executeJoinSelect(stmt, db)
	}

	// Simple SELECT without JOINs
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	// Build Projection
	var proj *projection.Projection
	var columns []string
	var metadata []ColumnMetadata

	// Check for SELECT *
	if len(stmt.Fields) == 1 && stmt.Fields[0].Value == "*" {
		proj = projection.NewProjection()
		// Get all columns from schema for result header
		for _, col := range table.Schema.Columns {
			columns = append(columns, col.Name)
			metadata = append(metadata, ColumnMetadata{
				Name: col.Name,
				Type: string(col.Type),
			})
		}
	} else {
		proj = &projection.Projection{
			SelectAll: false,
			Columns:   make([]projection.ColumnRef, len(stmt.Fields)),
		}
		for i, f := range stmt.Fields {
			// Handle qualified identifiers (table.column)
			if f.Table != "" {
				proj.Columns[i] = projection.ColumnRef{Table: f.Table, Column: f.Value}
			} else {
				proj.Columns[i] = projection.ColumnRef{Column: f.Value}
			}
			colName := f.String()
			columns = append(columns, colName)
			
			// Look up type from schema
			col := findColumnInSchema(table, f.Value)
			if col != nil {
				metadata = append(metadata, ColumnMetadata{
					Name: colName,
					Type: string(col.Type),
				})
			} else {
				metadata = append(metadata, ColumnMetadata{
					Name: colName,
					Type: "TEXT",
				})
			}
		}
	}

	var rows []data.Row

	if stmt.Where == nil {
		// Use domain model for SelectAll
		allRows := table.SelectAll()
		rows = make([]data.Row, len(allRows))
		for i, row := range allRows {
			rows[i] = projection.ProjectRow(row, proj, tableName)
		}
	} else {
		pred, err := predicate.Build(stmt.Where)
		if err != nil {
			return nil, err
		}
		// Use domain model for Select with predicate
		matchedRows := table.Select(pred)
		rows = make([]data.Row, len(matchedRows))
		for i, row := range matchedRows {
			rows[i] = projection.ProjectRow(row, proj, tableName)
		}
	}

	return &Result{
		Columns:  columns,
		Metadata: metadata,
		Rows:     rows,
		Message:  fmt.Sprintf("Returned %d rows", len(rows)),
	}, nil
}

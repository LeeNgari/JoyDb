package executor

import (
	"fmt"
	"sort"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// formatResult converts IntermediateResult to user-facing Result
func formatResult(intermediate *IntermediateResult, columns []string, metadata []ColumnMetadata) *Result {
	return &Result{
		Columns:  columns,
		Metadata: metadata,
		Rows:     intermediate.Rows,
		Message:  fmt.Sprintf("Returned %d rows", len(intermediate.Rows)),
	}
}

// formatInsertResult creates a Result for INSERT operations
func formatInsertResult(intermediate *IntermediateResult) *Result {
	rowsAffected, _ := intermediate.Metadata["rows_affected"].(int)
	if rowsAffected == 0 {
		rowsAffected = 1
	}

	return &Result{
		Message:      "INSERT 1",
		RowsAffected: rowsAffected,
	}
}

// formatUpdateResult creates a Result for UPDATE operations
func formatUpdateResult(intermediate *IntermediateResult) *Result {
	rowsAffected, _ := intermediate.Metadata["rows_affected"].(int)

	return &Result{
		Message:      fmt.Sprintf("UPDATE %d", rowsAffected),
		RowsAffected: rowsAffected,
	}
}

// formatDeleteResult creates a Result for DELETE operations
func formatDeleteResult(intermediate *IntermediateResult) *Result {
	rowsAffected, _ := intermediate.Metadata["rows_affected"].(int)

	return &Result{
		Message:      fmt.Sprintf("DELETE %d", rowsAffected),
		RowsAffected: rowsAffected,
	}
}

// formatSelectResult handles column and metadata calculation for SELECT queries
func formatSelectResult(node *plan.SelectNode, intermediate *IntermediateResult, db *schema.Database) *Result {
	var columns []string
	var metadata []ColumnMetadata

	proj := node.Projection
	
	// If it's a simple select (no joins), we can get types from the table
	table, hasTable := db.Tables[node.TableName]

	if proj.SelectAll {
		if hasTable && len(node.Children()) == 0 {
			// Simple SELECT *
			for _, col := range table.Schema.Columns {
				columns = append(columns, col.Name)
				metadata = append(metadata, ColumnMetadata{
					Name: col.Name,
					Type: string(col.Type),
				})
			}
		} else {
			// JOIN or complex result - extract from rows
			columns = extractColumnsFromRows(intermediate.Rows)
			for _, colName := range columns {
				metadata = append(metadata, ColumnMetadata{
					Name: colName,
					Type: "TEXT", // Default for complex results
				})
			}
		}
	} else {
		// Explicit projection
		for _, colRef := range proj.Columns {
			colName := colRef.Column
			if colRef.Alias != "" {
				colName = colRef.Alias
			} else if colRef.Table != "" {
				colName = fmt.Sprintf("%s.%s", colRef.Table, colRef.Column)
			}
			columns = append(columns, colName)

			// Try to find type if table is known
			var colType = "TEXT"
			if hasTable && colRef.Table == node.TableName {
				for _, c := range table.Schema.Columns {
					if c.Name == colRef.Column {
						colType = string(c.Type)
						break
					}
				}
			}
			
			metadata = append(metadata, ColumnMetadata{
				Name: colName,
				Type: colType,
			})
		}
	}

	return &Result{
		Columns:  columns,
		Metadata: metadata,
		Rows:     intermediate.Rows,
		Message:  fmt.Sprintf("Returned %d rows", len(intermediate.Rows)),
	}
}

// extractColumnsFromRows extracts column names from rows
// Used when columns aren't explicitly provided
func extractColumnsFromRows(rows []data.Row) []string {
	if len(rows) == 0 {
		return []string{}
	}

	// Get columns from first row and sort for consistency
	var columns []string
	for col := range rows[0].Data {
		columns = append(columns, col)
	}
	sort.Strings(columns)

	return columns
}

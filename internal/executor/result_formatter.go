package executor

import (
	"fmt"

	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/leengari/joydb/internal/plan"
)

// formatInsertResult creates a Result for INSERT operations
func formatInsertResult(intermediate *IntermediateResult) *Result {
	rowsAffected, _ := intermediate.Metadata["rows_affected"].(int)
	if rowsAffected == 0 {
		rowsAffected = 1
	}
	lastInsertID, _ := intermediate.Metadata["last_insert_id"].(int64)

	return &Result{
		Message:      "INSERT 1",
		RowsAffected: rowsAffected,
		LastInsertID: lastInsertID,
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
	if len(node.GroupBy) > 0 || len(node.Aggregates) > 0 {
		for _, column := range intermediate.Schema.Columns {
			columns = append(columns, column.Name)
			metadata = append(metadata, ColumnMetadata{Name: column.Name, Type: string(column.Type)})
		}
		return &Result{Columns: columns, Metadata: metadata, Rows: intermediate.Rows}
	}

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
			// JOIN or complex result - extract from schema
			if intermediate.Schema != nil {
				for _, col := range intermediate.Schema.Columns {
					columns = append(columns, col.Name)
					metadata = append(metadata, ColumnMetadata{
						Name: col.Name,
						Type: string(col.Type),
					})
				}
			}
		}
	} else {
		// Explicit projection
		for _, colRef := range proj.Columns {
			colName := colRef.Column
			if colRef.Alias != "" {
				colName = colRef.Alias
			}
			columns = append(columns, colName)

			// Find column type in table if possible
			colType := "TEXT" // Default
			if hasTable {
				col := findColumnInSchema(table, colRef.Column)
				if col != nil {
					colType = string(col.Type)
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
	}
}

// findColumnInSchema helps locate a column definition in a table schema
func findColumnInSchema(table *schema.Table, colName string) *schema.Column {
	for i := range table.Schema.Columns {
		if table.Schema.Columns[i].Name == colName {
			return &table.Schema.Columns[i]
		}
	}
	return nil
}

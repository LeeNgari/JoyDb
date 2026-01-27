package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

// executeSelect handles SELECT plans
func executeSelect(node *plan.SelectNode, db *schema.Database, tx *transaction.Transaction) (*Result, error) {
	// If there are JOINs, use the JOIN executor
	if len(node.Joins) > 0 {
		return executeJoinSelect(node, db, tx)
	}

	table, ok := db.Tables[node.TableName]
	if !ok {
		// Should be caught by planner, but check anyway
		return nil, fmt.Errorf("table not found: %s", node.TableName)
	}

	// Calculate Result Metadata (Columns & Types)
	var columns []string
	var metadata []ColumnMetadata

	proj := node.Projection
	if proj.SelectAll {
		// Get all columns from schema
		for _, col := range table.Schema.Columns {
			columns = append(columns, col.Name)
			metadata = append(metadata, ColumnMetadata{
				Name: col.Name,
				Type: string(col.Type),
			})
		}
	} else {
		for _, colRef := range proj.Columns {
			colName := colRef.Column
			if colRef.Alias != "" {
				colName = colRef.Alias
			} else if colRef.Table != "" {
				colName = fmt.Sprintf("%s.%s", colRef.Table, colRef.Column)
			}
			columns = append(columns, colName)

			// Look up type
			col := findColumnInSchema(table, colRef.Column)
			if col != nil {
				metadata = append(metadata, ColumnMetadata{
					Name: colName,
					Type: string(col.Type),
				})
			} else {
				metadata = append(metadata, ColumnMetadata{
					Name: colName,
					Type: "TEXT", // Fallback
				})
			}
		}
	}

	var rows []data.Row

	if node.Predicate == nil {
		allRows := table.SelectAll(tx)
		rows = make([]data.Row, len(allRows))
		for i, row := range allRows {
			rows[i] = projection.ProjectRow(row, proj, node.TableName)
		}
	} else {
		matchedRows := table.Select(node.Predicate, tx)
		rows = make([]data.Row, len(matchedRows))
		for i, row := range matchedRows {
			rows[i] = projection.ProjectRow(row, proj, node.TableName)
		}
	}

	return &Result{
		Columns:  columns,
		Metadata: metadata,
		Rows:     rows,
		Message:  fmt.Sprintf("Returned %d rows", len(rows)),
	}, nil
}

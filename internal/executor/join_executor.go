package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/query/operations/join"
)

// executeJoinSelect handles JOIN plans
func executeJoinSelect(node *plan.SelectNode, db *schema.Database, tx *transaction.Transaction) (*Result, error) {
	if len(node.Joins) != 1 {
		return nil, fmt.Errorf("multiple JOINs not yet supported")
	}

	joinNode := node.Joins[0]

	leftTableName := node.TableName
	leftTable, ok := db.Tables[leftTableName]
	if !ok {
		return nil, fmt.Errorf("left table not found: %s", leftTableName)
	}

	rightTableName := joinNode.TargetTable
	rightTable, ok := db.Tables[rightTableName]
	if !ok {
		return nil, fmt.Errorf("right table not found: %s", rightTableName)
	}

	// Build projection metadata
	var columns []string
	var metadata []ColumnMetadata

	proj := node.Projection

	if proj.SelectAll {
		// Get all columns from both tables
		for _, col := range leftTable.Schema.Columns {
			colName := leftTableName + "." + col.Name
			columns = append(columns, colName)
			metadata = append(metadata, ColumnMetadata{Name: colName, Type: string(col.Type)})
		}
		for _, col := range rightTable.Schema.Columns {
			colName := rightTableName + "." + col.Name
			columns = append(columns, colName)
			metadata = append(metadata, ColumnMetadata{Name: colName, Type: string(col.Type)})
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
			
			// Try to find column type
			var schemaCol *schema.Column
			if colRef.Table == leftTableName {
				schemaCol = findColumnInSchema(leftTable, colRef.Column)
			} else if colRef.Table == rightTableName {
				schemaCol = findColumnInSchema(rightTable, colRef.Column)
			} else {
				schemaCol = findColumnInSchema(leftTable, colRef.Column)
				if schemaCol == nil {
					schemaCol = findColumnInSchema(rightTable, colRef.Column)
				}
			}

			if schemaCol != nil {
				metadata = append(metadata, ColumnMetadata{Name: colName, Type: string(schemaCol.Type)})
			} else {
				metadata = append(metadata, ColumnMetadata{Name: colName, Type: "TEXT"})
			}
		}
	}

	var joinPred join.JoinPredicate
	if node.Predicate != nil {
		joinPred = func(row data.JoinedRow) bool {
			flatRow := make(data.Row)
			for k, v := range row.Data {
				flatRow[k] = v
			}
			return node.Predicate(flatRow)
		}
	}

	// Execute JOIN
	joinedRows, err := join.ExecuteJoin(
		leftTable,
		rightTable,
		joinNode.LeftOnCol,
		joinNode.RightOnCol,
		joinNode.JoinType,
		joinPred,
		node.Projection,
		tx,
	)
	if err != nil {
		return nil, fmt.Errorf("JOIN execution failed: %w", err)
	}

	// Convert JoinedRow to Row for Result
	rows := make([]data.Row, len(joinedRows))
	for i, joinedRow := range joinedRows {
		rows[i] = joinedRow.Data
	}

	return &Result{
		Columns:  columns,
		Metadata: metadata,
		Rows:     rows,
		Message:  fmt.Sprintf("Returned %d rows", len(rows)),
	}, nil
}

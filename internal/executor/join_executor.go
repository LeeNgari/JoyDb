package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/query/operations/join"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

// executeJoinSelect handles SELECT statements with JOINs
// Maps AST JOIN clauses to the engine's join.ExecuteJoin function
// Supports INNER, LEFT, RIGHT, and FULL OUTER JOINs
func executeJoinSelect(stmt *ast.SelectStatement, db *schema.Database) (*Result, error) {
	// Currently only supports single JOIN (can be extended for multiple JOINs)
	if len(stmt.Joins) != 1 {
		return nil, fmt.Errorf("multiple JOINs not yet supported (found %d)", len(stmt.Joins))
	}

	joinClause := stmt.Joins[0]

	// Get left table
	leftTableName := stmt.TableName.Value
	leftTable, ok := db.Tables[leftTableName]
	if !ok {
		return nil, fmt.Errorf("left table not found: %s", leftTableName)
	}

	// Get right table
	rightTableName := joinClause.RightTable.Value
	rightTable, ok := db.Tables[rightTableName]
	if !ok {
		return nil, fmt.Errorf("right table not found: %s", rightTableName)
	}

	// Parse JOIN condition to extract join columns
	// Expected format: leftTable.leftCol = rightTable.rightCol
	binExpr, ok := joinClause.OnCondition.(*ast.BinaryExpression)
	if !ok {
		return nil, fmt.Errorf("JOIN ON condition must be a comparison expression")
	}

	if binExpr.Operator != "=" {
		return nil, fmt.Errorf("JOIN ON condition must use = operator")
	}

	leftIdent, ok := binExpr.Left.(*ast.Identifier)
	if !ok {
		return nil, fmt.Errorf("left side of JOIN condition must be an identifier")
	}

	rightIdent, ok := binExpr.Right.(*ast.Identifier)
	if !ok {
		return nil, fmt.Errorf("right side of JOIN condition must be an identifier")
	}

	// Extract column names (handle qualified identifiers)
	leftJoinCol := leftIdent.Value
	rightJoinCol := rightIdent.Value

	// Convert JOIN type string to join.JoinType enum
	var joinType join.JoinType
	switch joinClause.JoinType {
	case "INNER":
		joinType = join.JoinTypeInner
	case "LEFT":
		joinType = join.JoinTypeLeft
	case "RIGHT":
		joinType = join.JoinTypeRight
	case "FULL":
		joinType = join.JoinTypeFull
	default:
		return nil, fmt.Errorf("unsupported JOIN type: %s", joinClause.JoinType)
	}

	// Build projection
	var proj *projection.Projection
	var columns []string

	if len(stmt.Fields) == 1 && stmt.Fields[0].Value == "*" {
		proj = projection.NewProjection()
		// Get all columns from both tables
		for _, col := range leftTable.Schema.Columns {
			columns = append(columns, leftTableName+"."+col.Name)
		}
		for _, col := range rightTable.Schema.Columns {
			columns = append(columns, rightTableName+"."+col.Name)
		}
	} else {
		proj = &projection.Projection{
			SelectAll: false,
			Columns:   make([]projection.ColumnRef, len(stmt.Fields)),
		}
		for i, f := range stmt.Fields {
			if f.Table != "" {
				proj.Columns[i] = projection.ColumnRef{Table: f.Table, Column: f.Value}
			} else {
				proj.Columns[i] = projection.ColumnRef{Column: f.Value}
			}
			columns = append(columns, f.String())
		}
	}

	// Build predicate if WHERE clause exists (convert to join.JoinPredicate)
	var pred join.JoinPredicate
	if stmt.Where != nil {
		crudPred, err := buildPredicate(stmt.Where)
		if err != nil {
			return nil, fmt.Errorf("failed to build WHERE predicate: %w", err)
		}
		// Convert crud.PredicateFunc to join.JoinPredicate
		pred = func(row data.JoinedRow) bool {
			// Flatten JoinedRow to regular Row for predicate evaluation
			flatRow := make(data.Row)
			for k, v := range row.Data {
				flatRow[k] = v
			}
			return crudPred(flatRow)
		}
	}

	// Execute JOIN using the engine
	joinedRows, err := join.ExecuteJoin(
		leftTable,
		rightTable,
		leftJoinCol,
		rightJoinCol,
		joinType,
		pred,
		proj,
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
		Columns: columns,
		Rows:    rows,
		Message: fmt.Sprintf("Returned %d rows", len(rows)),
	}, nil
}

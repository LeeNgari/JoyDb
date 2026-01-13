package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/query/operations/crud"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

type Result struct {
	Columns []string
	Rows    []data.Row
	Message string
}

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

	// Check for SELECT *
	if len(stmt.Fields) == 1 && stmt.Fields[0].Value == "*" {
		proj = projection.NewProjection()
		// Get all columns from schema for result header
		for _, col := range table.Schema.Columns {
			columns = append(columns, col.Name)
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
			columns = append(columns, f.String())
		}
	}

	var rows []data.Row

	if stmt.Where == nil {
		rows = crud.SelectAll(table, proj)
	} else {
		pred, err := buildPredicate(stmt.Where)
		if err != nil {
			return nil, err
		}
		rows = crud.SelectWhere(table, pred, proj)
	}

	return &Result{
		Columns: columns,
		Rows:    rows,
		Message: fmt.Sprintf("Returned %d rows", len(rows)),
	}, nil
}

func executeInsert(stmt *ast.InsertStatement, db *schema.Database) (*Result, error) {
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	if len(stmt.Columns) != len(stmt.Values) {
		return nil, fmt.Errorf("column count (%d) does not match value count (%d)", len(stmt.Columns), len(stmt.Values))
	}

	row := make(data.Row)
	for i, col := range stmt.Columns {
		valExpr := stmt.Values[i]
		lit, ok := valExpr.(*ast.Literal)
		if !ok {
			return nil, fmt.Errorf("only literal values supported in INSERT for now")
		}
		row[col.Value] = lit.Value
	}

	if err := crud.Insert(table, row); err != nil {
		return nil, err
	}

	return &Result{
		Message: "INSERT 1",
	}, nil
}

// executeUpdate handles UPDATE statements by converting AST to engine calls
// Maps UPDATE table SET col=val WHERE condition to crud.Update(table, predicate, updates)
// Example: UPDATE users SET email='new@test.com' WHERE id=5
func executeUpdate(stmt *ast.UpdateStatement, db *schema.Database) (*Result, error) {
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	// Convert AST expression map to data.Row (column -> value)
	// For now, we only support literal values in SET clause
	updates := make(data.Row)
	for colName, expr := range stmt.Updates {
		lit, ok := expr.(*ast.Literal)
		if !ok {
			return nil, fmt.Errorf("only literal values supported in SET clause for now")
		}
		updates[colName] = lit.Value
	}

	// Build predicate function from WHERE clause
	// If no WHERE clause, update all rows
	var pred crud.PredicateFunc
	if stmt.Where != nil {
		var err error
		pred, err = buildPredicate(stmt.Where)
		if err != nil {
			return nil, fmt.Errorf("failed to build WHERE predicate: %w", err)
		}
	} else {
		// No WHERE clause means update all rows
		pred = func(row data.Row) bool { return true }
	}

	// Call engine UPDATE operation
	count, err := crud.Update(table, pred, updates)
	if err != nil {
		return nil, fmt.Errorf("update failed: %w", err)
	}

	return &Result{
		Message: fmt.Sprintf("UPDATE %d", count),
	}, nil
}

// executeDelete handles DELETE statements by converting AST to engine calls
// Maps DELETE FROM table WHERE condition to crud.Delete(table, predicate)
// Example: DELETE FROM users WHERE active=false
func executeDelete(stmt *ast.DeleteStatement, db *schema.Database) (*Result, error) {
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	// Build predicate function from WHERE clause
	// If no WHERE clause, delete all rows (dangerous but allowed)
	var pred crud.PredicateFunc
	if stmt.Where != nil {
		var err error
		pred, err = buildPredicate(stmt.Where)
		if err != nil {
			return nil, fmt.Errorf("failed to build WHERE predicate: %w", err)
		}
	} else {
		// No WHERE clause means delete all rows
		pred = func(row data.Row) bool { return true }
	}

	// Call engine DELETE operation
	count, err := crud.Delete(table, pred)
	if err != nil {
		return nil, fmt.Errorf("delete failed: %w", err)
	}

	return &Result{
		Message: fmt.Sprintf("DELETE %d", count),
	}, nil
}

// buildPredicate converts an AST expression into a predicate function
// Supports:
//   - Comparison operators: =, <, >, <=, >=, !=, <>
//   - Logical operators: AND, OR
//   - Nested expressions with parentheses
// Returns a function that tests whether a row matches the condition
func buildPredicate(expr ast.Expression) (crud.PredicateFunc, error) {
	switch e := expr.(type) {
	case *ast.BinaryExpression:
		// Handle comparison expressions (col op value)
		return buildComparisonPredicate(e)
		
	case *ast.LogicalExpression:
		// Handle logical expressions (expr AND/OR expr)
		return buildLogicalPredicate(e)
		
	default:
		return nil, fmt.Errorf("unsupported expression type in WHERE clause: %T", expr)
	}
}

// buildComparisonPredicate builds a predicate for comparison expressions
func buildComparisonPredicate(binExpr *ast.BinaryExpression) (crud.PredicateFunc, error) {
	leftIdent, ok := binExpr.Left.(*ast.Identifier)
	if !ok {
		return nil, fmt.Errorf("left side of comparison must be an identifier")
	}

	rightLit, ok := binExpr.Right.(*ast.Literal)
	if !ok {
		return nil, fmt.Errorf("right side of comparison must be a literal")
	}

	colName := leftIdent.Value
	targetVal := rightLit.Value
	operator := binExpr.Operator

	return func(row data.Row) bool {
		val, ok := row[colName]
		if !ok {
			return false
		}
		
		// Use compareValues helper to handle all comparison operators
		return compareValues(val, operator, targetVal)
	}, nil
}

// buildLogicalPredicate builds a predicate for logical expressions (AND/OR)
// Recursively builds predicates for left and right sub-expressions
func buildLogicalPredicate(logExpr *ast.LogicalExpression) (crud.PredicateFunc, error) {
	// Recursively build predicates for left and right sides
	leftPred, err := buildPredicate(logExpr.Left)
	if err != nil {
		return nil, fmt.Errorf("failed to build left predicate: %w", err)
	}

	rightPred, err := buildPredicate(logExpr.Right)
	if err != nil {
		return nil, fmt.Errorf("failed to build right predicate: %w", err)
	}

	// Combine predicates based on operator
	if logExpr.Operator == "AND" {
		return func(row data.Row) bool {
			return leftPred(row) && rightPred(row)
		}, nil
	} else if logExpr.Operator == "OR" {
		return func(row data.Row) bool {
			return leftPred(row) || rightPred(row)
		}, nil
	}

	return nil, fmt.Errorf("unsupported logical operator: %s", logExpr.Operator)
}

// compareValues compares two values using the specified operator
// Handles numeric, string, and boolean comparisons
// Supports: =, <, >, <=, >=, !=, <>
func compareValues(left interface{}, op string, right interface{}) bool {
	// Try numeric comparison first
	if n1, ok := normalizeToFloat(left); ok {
		if n2, ok := normalizeToFloat(right); ok {
			switch op {
			case "=":
				return n1 == n2
			case "!=", "<>":
				return n1 != n2
			case "<":
				return n1 < n2
			case ">":
				return n1 > n2
			case "<=":
				return n1 <= n2
			case ">=":
				return n1 >= n2
			}
		}
	}
	
	// Try string comparison
	if s1, ok := left.(string); ok {
		if s2, ok := right.(string); ok {
			switch op {
			case "=":
				return s1 == s2
			case "!=", "<>":
				return s1 != s2
			case "<":
				return s1 < s2
			case ">":
				return s1 > s2
			case "<=":
				return s1 <= s2
			case ">=":
				return s1 >= s2
			}
		}
	}
	
	// Fallback: direct equality/inequality comparison for booleans and other types
	switch op {
	case "=":
		return left == right
	case "!=", "<>":
		return left != right
	default:
		// For non-comparable types with ordering operators, return false
		return false
	}
}

func normalizeToFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case float64:
		return val, true
	}
	return 0, false
}

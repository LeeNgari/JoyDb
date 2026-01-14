package predicate

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/util/types"
)

// PredicateFunc is a function that tests whether a row matches certain criteria
type PredicateFunc func(data.Row) bool

// Build converts an AST expression into a predicate function
// Supports:
//   - Comparison operators: =, <, >, <=, >=, !=, <>
//   - Logical operators: AND, OR
//   - Nested expressions with parentheses
// Returns a function that tests whether a row matches the condition
func Build(expr ast.Expression) (PredicateFunc, error) {
	switch e := expr.(type) {
	case *ast.BinaryExpression:
		// Handle comparison expressions (col op value)
		return buildComparison(e)
		
	case *ast.LogicalExpression:
		// Handle logical expressions (expr AND/OR expr)
		return buildLogical(e)
		
	default:
		return nil, fmt.Errorf("unsupported expression type in WHERE clause: %T", expr)
	}
}

// buildComparison builds a predicate for comparison expressions
func buildComparison(binExpr *ast.BinaryExpression) (PredicateFunc, error) {
	leftIdent, ok := binExpr.Left.(*ast.Identifier)
	if !ok {
		return nil, fmt.Errorf("left side of comparison must be an identifier")
	}

	rightLit, ok := binExpr.Right.(*ast.Literal)
	if !ok {
		return nil, fmt.Errorf("right side of comparison must be a literal")
	}

	// Get column name (may be qualified like "orders.amount" or unqualified like "amount")
	colName := leftIdent.Value
	tableName := leftIdent.Table
	operator := binExpr.Operator
	targetVal := rightLit.Value

	return func(row data.Row) bool {
		var val interface{}
		var ok bool

		// Try qualified name first if table is specified (e.g., "orders.amount")
		if tableName != "" {
			qualifiedName := tableName + "." + colName
			val, ok = row[qualifiedName]
		}

		// If not found with qualified name, try unqualified (e.g., "amount")
		if !ok {
			val, ok = row[colName]
		}

		// If still not found, return false
		if !ok {
			return false
		}
		
		// Use types.CompareValues to handle all comparison operators
		return types.CompareValues(val, operator, targetVal)
	}, nil
}

// buildLogical builds a predicate for logical expressions (AND/OR)
// Recursively builds predicates for left and right sub-expressions
func buildLogical(logExpr *ast.LogicalExpression) (PredicateFunc, error) {
	// Recursively build predicates for left and right sides
	leftPred, err := Build(logExpr.Left)
	if err != nil {
		return nil, fmt.Errorf("failed to build left predicate: %w", err)
	}

	rightPred, err := Build(logExpr.Right)
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

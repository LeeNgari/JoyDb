package predicate

import (
	"fmt"

	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/leengari/joydb/internal/parser/ast"
	"github.com/leengari/joydb/internal/util/types"
)

// PredicateFunc is a function that tests whether a row matches certain criteria
type PredicateFunc func(data.Row) bool

// Build converts an AST expression into a predicate function
// Supports:
//   - Comparison operators: =, <, >, <=, >=, !=, <>
//   - Logical operators: AND, OR
//   - Nested expressions with parentheses
// Returns a function that tests whether a row matches the condition
func Build(expr ast.Expression, tableSchema *schema.TableSchema) (PredicateFunc, error) {
	switch e := expr.(type) {
	case *ast.BinaryExpression:
		// Handle comparison expressions (col op value)
		return buildComparison(e, tableSchema)
		
	case *ast.LogicalExpression:
		// Handle logical expressions (expr AND/OR expr)
		return buildLogical(e, tableSchema)
		
	default:
		return nil, fmt.Errorf("unsupported expression type in WHERE clause: %T", expr)
	}
}

// buildComparison builds a predicate for comparison expressions
func buildComparison(binExpr *ast.BinaryExpression, tableSchema *schema.TableSchema) (PredicateFunc, error) {
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
	operator := binExpr.Operator
	targetVal := rightLit.Value

	colIdx := -1
	if tableSchema != nil {
		colIdx = tableSchema.GetColumnIndex(colName)
		if colIdx == -1 {
			// If not found, maybe the AST has a table alias/prefix that we should ignore for now
			// in a single-table context. For multi-table, this gets complex, but JoyDb joins
			// currently handle this differently or strip it in planning.
			// Let's just try to match without table prefix if it has one.
			// Currently JoyDb parser outputs leftIdent.Value as the column name.
			return nil, fmt.Errorf("column %q not found in schema", colName)
		}
	} else {
		return nil, fmt.Errorf("schema required to build predicate")
	}

	return func(row data.Row) bool {
		if colIdx == -1 || colIdx >= len(row.Values) {
			return false
		}
		val := row.Values[colIdx]
		if val == nil {
			return false
		}
		
		// Use types.CompareValues to handle all comparison operators
		return types.CompareValues(val, operator, targetVal)
	}, nil
}

// buildLogical builds a predicate for logical expressions (AND/OR)
// Recursively builds predicates for left and right sub-expressions
func buildLogical(logExpr *ast.LogicalExpression, tableSchema *schema.TableSchema) (PredicateFunc, error) {
	// Recursively build predicates for left and right sides
	leftPred, err := Build(logExpr.Left, tableSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to build left predicate: %w", err)
	}

	rightPred, err := Build(logExpr.Right, tableSchema)
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

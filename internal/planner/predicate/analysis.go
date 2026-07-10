package predicate

import (
	"github.com/leengari/mini-rdbms/internal/parser/ast"
)

// PredicateInfo holds structured metadata extracted from a WHERE clause
type PredicateInfo struct {
	Column   string      // e.g. "id"
	Table    string      // e.g. "users" (may be empty)
	Operator string      // "=", "<", ">", "<=", ">="
	Value    interface{} // The literal value
}

// AnalyzeForIndexScan inspects an AST Expression and returns
// structured info if it's a simple "column op literal" comparison.
// Returns nil if the expression is too complex or not suitable for index use.
func AnalyzeForIndexScan(expr ast.Expression) *PredicateInfo {
	if expr == nil {
		return nil
	}

	binExpr, ok := expr.(*ast.BinaryExpression)
	if !ok {
		return nil
	}

	leftIdent, leftOk := binExpr.Left.(*ast.Identifier)
	rightLit, rightOk := binExpr.Right.(*ast.Literal)

	if leftOk && rightOk {
		op := binExpr.Operator
		if op == "=" || op == "<" || op == ">" || op == "<=" || op == ">=" {
			return &PredicateInfo{
				Column:   leftIdent.Value,
				Table:    leftIdent.Table,
				Operator: op,
				Value:    rightLit.Value,
			}
		}
	}

	return nil
}

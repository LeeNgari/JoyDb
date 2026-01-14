package ast

import "fmt"

// BinaryExpression: Left Operator Right (e.g. id = 1)
type BinaryExpression struct {
	Left     Expression
	Operator string
	Right    Expression
}

func (e *BinaryExpression) expressionNode()      {}
func (e *BinaryExpression) TokenLiteral() string { return e.Operator }
func (e *BinaryExpression) String() string {
	return fmt.Sprintf("(%s %s %s)", e.Left.String(), e.Operator, e.Right.String())
}

// LogicalExpression: Left Operator Right (e.g. age > 18 AND active = true)
// Represents logical operations (AND, OR) that combine multiple conditions
// Used in WHERE clauses to create complex predicates
type LogicalExpression struct {
	Left     Expression
	Operator string // "AND" or "OR"
	Right    Expression
}

func (e *LogicalExpression) expressionNode()      {}
func (e *LogicalExpression) TokenLiteral() string { return e.Operator }
func (e *LogicalExpression) String() string {
	return fmt.Sprintf("(%s %s %s)", e.Left.String(), e.Operator, e.Right.String())
}

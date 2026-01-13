package ast

import (
	"bytes"
	"fmt"
)

// Node is the base interface for all AST nodes
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement represents a standalone SQL statement (SELECT, INSERT, etc.)
type Statement interface {
	Node
	statementNode()
}

// Expression represents a value or operation
type Expression interface {
	Node
	expressionNode()
}

// Identifier represents a column or table name
// Can be qualified (table.column) or unqualified (column)
type Identifier struct {
	TokenLiteralValue string // The token literal (e.g. "users" or "users.id")
	Value             string // The column/table name (e.g. "users" or "id")
	Table             string // Optional table qualifier (e.g. "users" in "users.id")
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.TokenLiteralValue }
func (i *Identifier) String() string {
	if i.Table != "" {
		return i.Table + "." + i.Value
	}
	return i.Value
}

// Literal represents a fixed value (string, number)
type Literal struct {
	TokenLiteralValue string
	Value             interface{} // string, int, float64
	Kind              int         // 0=String, 1=Int, 2=Float
}

func (l *Literal) expressionNode()      {}
func (l *Literal) TokenLiteral() string { return l.TokenLiteralValue }
func (l *Literal) String() string       { return l.TokenLiteralValue }

// SelectStatement: SELECT col1, col2 FROM table WHERE ...
type SelectStatement struct {
	Fields    []*Identifier
	TableName *Identifier
	Where     Expression // For now, simple binary expression or nil
}

func (s *SelectStatement) statementNode()       {}
func (s *SelectStatement) TokenLiteral() string { return "SELECT" }
func (s *SelectStatement) String() string {
	var out bytes.Buffer
	out.WriteString("SELECT ")
	for i, f := range s.Fields {
		out.WriteString(f.String())
		if i < len(s.Fields)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(" FROM ")
	out.WriteString(s.TableName.String())
	if s.Where != nil {
		out.WriteString(" WHERE ")
		out.WriteString(s.Where.String())
	}
	return out.String()
}

// InsertStatement: INSERT INTO table (col1, col2) VALUES (val1, val2)
type InsertStatement struct {
	TableName *Identifier
	Columns   []*Identifier
	Values    []Expression
}

func (s *InsertStatement) statementNode()       {}
func (s *InsertStatement) TokenLiteral() string { return "INSERT" }
func (s *InsertStatement) String() string {
	var out bytes.Buffer
	out.WriteString("INSERT INTO ")
	out.WriteString(s.TableName.String())
	out.WriteString(" (")
	for i, c := range s.Columns {
		out.WriteString(c.String())
		if i < len(s.Columns)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(") VALUES (")
	for i, v := range s.Values {
		out.WriteString(v.String())
		if i < len(s.Values)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(")")
	return out.String()
}

// UpdateStatement: UPDATE table SET col1 = val1, col2 = val2 WHERE ...
// Represents an UPDATE SQL statement that modifies existing rows in a table.
// The Updates map contains column names as keys and their new values as expressions.
// WHERE clause is optional - if nil, all rows will be updated.
type UpdateStatement struct {
	TableName *Identifier
	Updates   map[string]Expression // column name -> new value expression
	Where     Expression            // optional predicate
}

func (s *UpdateStatement) statementNode()       {}
func (s *UpdateStatement) TokenLiteral() string { return "UPDATE" }
func (s *UpdateStatement) String() string {
	var out bytes.Buffer
	out.WriteString("UPDATE ")
	out.WriteString(s.TableName.String())
	out.WriteString(" SET ")
	
	// Note: map iteration order is non-deterministic, but that's okay for debugging
	first := true
	for col, val := range s.Updates {
		if !first {
			out.WriteString(", ")
		}
		out.WriteString(col)
		out.WriteString(" = ")
		out.WriteString(val.String())
		first = false
	}
	
	if s.Where != nil {
		out.WriteString(" WHERE ")
		out.WriteString(s.Where.String())
	}
	return out.String()
}

// DeleteStatement: DELETE FROM table WHERE ...
// Represents a DELETE SQL statement that removes rows from a table.
// WHERE clause is optional - if nil, all rows will be deleted.
type DeleteStatement struct {
	TableName *Identifier
	Where     Expression // optional predicate
}

func (s *DeleteStatement) statementNode()       {}
func (s *DeleteStatement) TokenLiteral() string { return "DELETE" }
func (s *DeleteStatement) String() string {
	var out bytes.Buffer
	out.WriteString("DELETE FROM ")
	out.WriteString(s.TableName.String())
	if s.Where != nil {
		out.WriteString(" WHERE ")
		out.WriteString(s.Where.String())
	}
	return out.String()
}

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

package ast

import "bytes"

// SelectStatement: SELECT fields FROM table [JOIN ...] [WHERE condition]
// Represents a SELECT SQL query with optional JOINs and WHERE clause
type SelectStatement struct {
	Fields    []*Identifier
	TableName *Identifier
	Joins     []*JoinClause // Optional JOIN clauses
	Where     Expression    // Optional WHERE clause
}

func (s *SelectStatement) statementNode()       {}
func (s *SelectStatement) TokenLiteral() string { return "SELECT" }
func (s *SelectStatement) String() string {
	var out bytes.Buffer
	out.WriteString("SELECT ")
	for i, f := range s.Fields {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(f.String())
	}
	out.WriteString(" FROM ")
	out.WriteString(s.TableName.String())
	
	// Add JOINs if present
	for _, join := range s.Joins {
		out.WriteString(" ")
		out.WriteString(join.String())
	}
	
	if s.Where != nil {
		out.WriteString(" WHERE ")
		out.WriteString(s.Where.String())
	}
	return out.String()
}

// JoinClause represents a JOIN operation in a SELECT statement
// Example: INNER JOIN orders ON users.id = orders.user_id
type JoinClause struct {
	JoinType    string      // "INNER", "LEFT", "RIGHT", "FULL"
	RightTable  *Identifier // Table to join with
	OnCondition Expression  // JOIN condition (e.g., users.id = orders.user_id)
}

func (j *JoinClause) String() string {
	var out bytes.Buffer
	out.WriteString(j.JoinType)
	out.WriteString(" JOIN ")
	out.WriteString(j.RightTable.String())
	out.WriteString(" ON ")
	out.WriteString(j.OnCondition.String())
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

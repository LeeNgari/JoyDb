package ast

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

// LiteralKind represents the type of a literal value
type LiteralKind string

const (
	LiteralString LiteralKind = "STRING"
	LiteralInt    LiteralKind = "INT"
	LiteralFloat  LiteralKind = "FLOAT"
	LiteralBool   LiteralKind = "BOOL"
	LiteralDate   LiteralKind = "DATE"
	LiteralTime   LiteralKind = "TIME"
	LiteralEmail  LiteralKind = "EMAIL"
)

// Literal represents a fixed value (string, number, boolean, date, time, email)
// Examples: 'hello', 42, 3.14, true, DATE '2024-01-13', TIME '14:30:00', EMAIL 'user@example.com'
type Literal struct {
	TokenLiteralValue string      // The original token text
	Value             interface{} // The parsed value (string, int, float64, bool)
	Kind              LiteralKind // The type of literal
}

func (l *Literal) expressionNode()      {}
func (l *Literal) TokenLiteral() string { return l.TokenLiteralValue }
func (l *Literal) String() string       { return l.TokenLiteralValue }

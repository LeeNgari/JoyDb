package ast

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

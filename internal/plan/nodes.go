package plan

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/query/operations/join"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

// Node is the base interface for all execution plan nodes
type Node interface {
	// Children returns child nodes for tree walking
	Children() []Node
	
	// Metadata returns attached metadata (never nil)
	Metadata() map[string]any
	
	// NodeType returns the type identifier (for debugging/logging)
	NodeType() string
}

// ScanNode represents a table scan operation (leaf node)
type ScanNode struct {
	TableName   string
	Predicate   func(data.Row) bool
	Transaction *transaction.Transaction
	
	metadata map[string]any
}

func (n *ScanNode) Children() []Node {
	return nil // Leaf node has no children
}

func (n *ScanNode) Metadata() map[string]any {
	if n.metadata == nil {
		n.metadata = make(map[string]any)
	}
	return n.metadata
}

func (n *ScanNode) NodeType() string {
	return "SCAN"
}

// JoinNode represents a JOIN operation (composite node with two children)
type JoinNode struct {
	JoinType    join.JoinType
	LeftOnCol   string
	RightOnCol  string
	
	// Tree structure - JOIN has two children
	left  Node
	right Node
	
	metadata map[string]any
}

func NewJoinNode(left, right Node, joinType join.JoinType, leftCol, rightCol string) *JoinNode {
	return &JoinNode{
		left:       left,
		right:      right,
		JoinType:   joinType,
		LeftOnCol:  leftCol,
		RightOnCol: rightCol,
	}
}

func (n *JoinNode) Left() Node {
	return n.left
}

func (n *JoinNode) Right() Node {
	return n.right
}

func (n *JoinNode) Children() []Node {
	return []Node{n.left, n.right}
}

func (n *JoinNode) Metadata() map[string]any {
	if n.metadata == nil {
		n.metadata = make(map[string]any)
	}
	return n.metadata
}

func (n *JoinNode) NodeType() string {
	return "JOIN"
}

// SelectNode represents a SELECT operation
type SelectNode struct {
	TableName string
	// Predicate filters rows. If nil, all rows are selected.
	Predicate func(data.Row) bool
	// Projection defines which columns to return.
	Projection *projection.Projection
	// Transaction context
	Transaction *transaction.Transaction
	
	// Tree structure - children are JOINs or other operations
	children []Node
	
	metadata map[string]any
}

func (n *SelectNode) Children() []Node {
	return n.children
}

func (n *SelectNode) AddChild(child Node) {
	n.children = append(n.children, child)
}

func (n *SelectNode) Metadata() map[string]any {
	if n.metadata == nil {
		n.metadata = make(map[string]any)
	}
	return n.metadata
}

func (n *SelectNode) NodeType() string {
	return "SELECT"
}

// InsertNode represents an INSERT operation
type InsertNode struct {
	TableName string
	Row       data.Row // The row to insert (already parsed/converted)
	// Transaction context
	Transaction *transaction.Transaction
	
	children []Node
	metadata map[string]any
}

func (n *InsertNode) Children() []Node {
	return n.children
}

func (n *InsertNode) Metadata() map[string]any {
	if n.metadata == nil {
		n.metadata = make(map[string]any)
	}
	return n.metadata
}

func (n *InsertNode) NodeType() string {
	return "INSERT"
}

// UpdateNode represents an UPDATE operation
type UpdateNode struct {
	TableName string
	Predicate func(data.Row) bool
	Updates   data.Row // Map of columns to update
	// Transaction context
	Transaction *transaction.Transaction
	
	children []Node
	metadata map[string]any
}

func (n *UpdateNode) Children() []Node {
	return n.children
}

func (n *UpdateNode) Metadata() map[string]any {
	if n.metadata == nil {
		n.metadata = make(map[string]any)
	}
	return n.metadata
}

func (n *UpdateNode) NodeType() string {
	return "UPDATE"
}

// DeleteNode represents a DELETE operation
type DeleteNode struct {
	TableName string
	Predicate func(data.Row) bool
	// Transaction context
	Transaction *transaction.Transaction
	
	children []Node
	metadata map[string]any
}

func (n *DeleteNode) Children() []Node {
	return n.children
}

func (n *DeleteNode) Metadata() map[string]any {
	if n.metadata == nil {
		n.metadata = make(map[string]any)
	}
	return n.metadata
}

func (n *DeleteNode) NodeType() string {
	return "DELETE"
}

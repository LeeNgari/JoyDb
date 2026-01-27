package plan

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/query/operations/join"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

// NodeType identifies the type of plan node
type NodeType int

const (
	NodeSelect NodeType = iota
	NodeInsert
	NodeUpdate
	NodeDelete
)

// Node is the interface for all execution plan nodes
type Node interface {
	Type() NodeType
}

// SelectNode represents a SELECT operation
type SelectNode struct {
	TableName string
	// Predicate filters rows. If nil, all rows are selected.
	Predicate func(data.Row) bool
	// Projection defines which columns to return.
	Projection *projection.Projection
	// Joins defines any joins to perform
	Joins []JoinNode
	// Transaction context
	Transaction *transaction.Transaction
}

func (n *SelectNode) Type() NodeType { return NodeSelect }

// JoinNode represents a JOIN operation within a SELECT
type JoinNode struct {
	TargetTable string
	JoinType    join.JoinType
	LeftOnCol   string
	RightOnCol  string
}

// InsertNode represents an INSERT operation
type InsertNode struct {
	TableName string
	Row       data.Row // The row to insert (already parsed/converted)
	// Transaction context
	Transaction *transaction.Transaction
}

func (n *InsertNode) Type() NodeType { return NodeInsert }

// UpdateNode represents an UPDATE operation
type UpdateNode struct {
	TableName string
	Predicate func(data.Row) bool
	Updates   data.Row // Map of columns to update
	// Transaction context
	Transaction *transaction.Transaction
}

func (n *UpdateNode) Type() NodeType { return NodeUpdate }

// DeleteNode represents a DELETE operation
type DeleteNode struct {
	TableName string
	Predicate func(data.Row) bool
	// Transaction context
	Transaction *transaction.Transaction
}

func (n *DeleteNode) Type() NodeType { return NodeDelete }

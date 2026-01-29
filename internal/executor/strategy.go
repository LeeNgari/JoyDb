package executor

import (
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// ExecutionStrategy defines how a plan node is executed
// Scaffold: Interface defined but not used yet
type ExecutionStrategy interface {
	Execute(node plan.Node, ctx *ExecutionContext) (*Result, error)
}

// ExecutionContext provides resources for execution
type ExecutionContext struct {
	Database    *schema.Database
	Transaction *transaction.Transaction
	Config      *ExecutionConfig
}

// ExecutionConfig holds execution parameters
type ExecutionConfig struct {
	UseIndexes    bool
	ParallelScans bool
	JoinAlgorithm string // "hash", "nested_loop", "merge"
	BufferSize    int
}

// DefaultExecutionConfig returns default configuration
func DefaultExecutionConfig() *ExecutionConfig {
	return &ExecutionConfig{
		UseIndexes:    false, // Scaffold: always false
		ParallelScans: false,
		JoinAlgorithm: "nested_loop", // Scaffold: always nested loop
		BufferSize:    4096,
	}
}

// DefaultStrategy is a naive implementation
// Integrated with tree-walking executor
type DefaultStrategy struct{}

func (ds *DefaultStrategy) Execute(node plan.Node, ctx *ExecutionContext) (*IntermediateResult, error) {
	// Delegate to the recursive tree walker (implemented in executor.go)
	return executeNode(node, ctx)
}

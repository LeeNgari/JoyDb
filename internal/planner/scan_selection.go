package planner

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// selectScanType determines whether to use index or sequential scan
// Scaffold: Always returns "sequential" (naive implementation)
func selectScanType(tableName string, predicate func(data.Row) bool, db *schema.Database) string {
	// Future: Implement real scan selection based on:
	// - Index availability
	// - Predicate selectivity
	// - Table size
	// - Index statistics
	return "sequential"
}

// selectJoinAlgorithm determines which join algorithm to use
// Scaffold: Always returns "nested_loop" (naive implementation)
func selectJoinAlgorithm(joinNode *plan.JoinNode, db *schema.Database) string {
	// Future: Implement real join algorithm selection based on:
	// - Table sizes
	// - Available memory
	// - Join selectivity
	// - Index availability
	// Options: "nested_loop", "hash", "merge"
	return "nested_loop"
}

// shouldUseIndex determines if an index should be used for a predicate
// Scaffold: Always returns false (naive implementation)
func shouldUseIndex(tableName string, predicate func(data.Row) bool, db *schema.Database) bool {
	// Future: Implement real index selection based on:
	// - Index availability
	// - Predicate analysis
	// - Index selectivity
	return false
}

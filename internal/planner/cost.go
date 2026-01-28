package planner

import "github.com/leengari/mini-rdbms/internal/plan"

// estimateCost estimates the cost of executing a plan node
// Scaffold: Always returns 1.0 (naive implementation)
func estimateCost(node plan.Node) float64 {
	// Future: Implement real cost estimation based on:
	// - Table sizes
	// - Index availability
	// - Join selectivity
	// - Predicate selectivity
	return 1.0
}

// attachCostEstimate attaches cost metadata to a node
// Scaffold: Attaches naive cost estimate
func attachCostEstimate(node plan.Node) {
	cost := estimateCost(node)
	node.Metadata()["estimated_cost"] = cost
	node.Metadata()["cost_estimated"] = true
}

// estimateRowCount estimates the number of rows a node will return
// Scaffold: Always returns 1000 (naive implementation)
func estimateRowCount(node plan.Node) int64 {
	// Future: Implement real row count estimation
	return 1000
}

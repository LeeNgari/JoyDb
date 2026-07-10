package benchmark

import (
	"github.com/leengari/mini-rdbms/internal/engine"
)

// Workload defines a specific database operation or sequence of operations to be benchmarked.
// It provides lifecycle hooks to set up required schema/data, run the operation, and tear it down.
type Workload interface {
	// Name returns the identifier for this workload (e.g., "insert_single_row")
	Name() string

	// Description returns a human-readable explanation of what is being measured
	Description() string

	// Tags returns a list of categories this workload belongs to (e.g., "write", "read", "join")
	Tags() []string

	// Setup is called once before any measurements begin.
	// Use this to create tables, build indexes, and seed initial data.
	Setup(eng *engine.Engine) error

	// Run executes a single iteration of the benchmark.
	// This is the only method that is timed. The iter parameter is the current iteration number.
	Run(eng *engine.Engine, iter int) error

	// Teardown is called once after all measurements are complete.
	// Use this to drop tables or clean up resources.
	Teardown(eng *engine.Engine) error
}

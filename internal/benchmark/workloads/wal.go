package workloads

import (
	"time"

	"github.com/leengari/mini-rdbms/internal/benchmark"
)

// InsertWithWAL measures insert throughput with WAL enabled.
// Because the harness sets up the engine before calling Setup, we need to create
// two separate workloads and configure the harness to use the appropriate EngineOptions.
// However, the harness takes a Workload interface. To switch WAL on/off, we can
// define a method on the workload to provide custom EngineOptions, but our interface
// doesn't have that yet.
// A better approach is to configure the harness from outside (e.g. in bench_test.go or main.go).
// These structs are just thin wrappers around InsertSingleRow.

type InsertWithWAL struct {
	*InsertSingleRow
}

func NewInsertWithWAL() *InsertWithWAL {
	return &InsertWithWAL{InsertSingleRow: NewInsertSingleRow()}
}

func (w *InsertWithWAL) Name() string        { return "insert_with_wal" }
func (w *InsertWithWAL) Description() string { return "Insert single row with WAL enabled." }
func (w *InsertWithWAL) Tags() []string      { return []string{"write", "wal"} }

type InsertWithPeriodicWAL struct {
	*InsertSingleRow
}

func NewInsertWithPeriodicWAL() *InsertWithPeriodicWAL {
	return &InsertWithPeriodicWAL{InsertSingleRow: NewInsertSingleRow()}
}

func (w *InsertWithPeriodicWAL) Name() string { return "insert_wal_async_100ms" }
func (w *InsertWithPeriodicWAL) Description() string {
	return "Insert single row with WAL syncing every 100ms."
}
func (w *InsertWithPeriodicWAL) Tags() []string { return []string{"write", "wal", "async"} }

func (w *InsertWithPeriodicWAL) GetEngineOptions() benchmark.EngineOptions {
	opts := benchmark.DefaultEngineOptions()
	opts.WALEnabled = true
	opts.WALSyncInterval = 100 * time.Millisecond
	return opts
}

type InsertNoWAL struct {
	*InsertSingleRow
}

func NewInsertNoWAL() *InsertNoWAL {
	return &InsertNoWAL{InsertSingleRow: NewInsertSingleRow()}
}

func (w *InsertNoWAL) Name() string        { return "insert_no_wal" }
func (w *InsertNoWAL) Description() string { return "Insert single row with WAL disabled." }
func (w *InsertNoWAL) Tags() []string      { return []string{"write", "no-wal"} }

// GetEngineOptions is an optional interface the harness can check
type CustomEngineOptionsProvider interface {
	GetEngineOptions() benchmark.EngineOptions
}

func (w *InsertWithWAL) GetEngineOptions() benchmark.EngineOptions {
	opts := benchmark.DefaultEngineOptions()
	opts.WALEnabled = true
	return opts
}

func (w *InsertNoWAL) GetEngineOptions() benchmark.EngineOptions {
	opts := benchmark.DefaultEngineOptions()
	opts.WALEnabled = false
	return opts
}

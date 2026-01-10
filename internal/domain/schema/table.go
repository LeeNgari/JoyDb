package schema

import (
	"sync"

	"github.com/leengari/mini-rdbms/internal/domain/data"
)

// Table represents a database table with its schema, data, and indexes
// Note: Table operations are thread-safe via mutex
type Table struct {
	mu           sync.RWMutex // protects all fields below
	Name         string
	Path         string // filesystem path to table directory
	Schema       *TableSchema
	Rows         []data.Row
	Indexes      map[string]*data.Index
	LastInsertID int64
	Dirty        bool // tracks if table has unsaved changes
}

// MarkDirty marks the table as having unsaved changes
// This should be called after any mutation operation (INSERT, UPDATE, DELETE)
func (t *Table) MarkDirty() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.MarkDirtyUnsafe()
}

// MarkDirtyUnsafe sets dirty flag without acquiring lock
// IMPORTANT: Only call this when you already hold the table lock!
// Use MarkDirty() if you don't hold the lock.
func (t *Table) MarkDirtyUnsafe() {
	t.Dirty = true
}

// Lock acquires an exclusive lock on the table for write operations
func (t *Table) Lock() {
	t.mu.Lock()
}

// Unlock releases the exclusive lock
func (t *Table) Unlock() {
	t.mu.Unlock()
}

// RLock acquires a read lock on the table for read operations
func (t *Table) RLock() {
	t.mu.RLock()
}

// RUnlock releases the read lock
func (t *Table) RUnlock() {
	t.mu.RUnlock()
}

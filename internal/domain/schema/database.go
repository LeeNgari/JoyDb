package schema

import "sync"

// Database represents a single database on disk
// (a directory containing table subdirectories)
type Database struct {
	mu     sync.RWMutex
	Name   string
	Path   string // filesystem path to database directory
	Tables map[string]*Table
}

// Lock acquires an exclusive lock on the database
func (d *Database) Lock() {
	d.mu.Lock()
}

// Unlock releases the exclusive lock on the database
func (d *Database) Unlock() {
	d.mu.Unlock()
}

// RLock acquires a read lock on the database
func (d *Database) RLock() {
	d.mu.RLock()
}

// RUnlock releases the read lock on the database
func (d *Database) RUnlock() {
	d.mu.RUnlock()
}

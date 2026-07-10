package engine

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/leengari/joydb/internal/domain/transaction"
)

// InMemoryEngine stores database catalogs entirely in memory.
type InMemoryEngine struct {
	mu        sync.RWMutex
	databases map[string]*schema.Database
}

func NewInMemoryEngine() *InMemoryEngine {
	return &InMemoryEngine{databases: make(map[string]*schema.Database)}
}

func (e *InMemoryEngine) LoadDatabase(dbPath string) (*schema.Database, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	name := strings.TrimPrefix(filepath.Base(dbPath), ":memory:")
	db, ok := e.databases[name]
	if !ok {
		return nil, fmt.Errorf("database '%s' does not exist", name)
	}
	return db, nil
}

func (e *InMemoryEngine) SaveDatabase(*schema.Database, *transaction.Transaction) error { return nil }

func (e *InMemoryEngine) CreateDatabase(name, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.databases[name]; exists {
		return fmt.Errorf("database '%s' already exists", name)
	}
	e.databases[name] = &schema.Database{Name: name, Path: ":memory:", Tables: make(map[string]*schema.Table)}
	return nil
}

func (e *InMemoryEngine) DropDatabase(name, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.databases[name]; !exists {
		return fmt.Errorf("database '%s' does not exist", name)
	}
	delete(e.databases, name)
	return nil
}

func (e *InMemoryEngine) RenameDatabase(oldName, newName, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	db, exists := e.databases[oldName]
	if !exists {
		return fmt.Errorf("database '%s' does not exist", oldName)
	}
	if _, exists := e.databases[newName]; exists {
		return fmt.Errorf("database '%s' already exists", newName)
	}
	delete(e.databases, oldName)
	db.Name = newName
	e.databases[newName] = db
	return nil
}

func (e *InMemoryEngine) ListDatabases(_ string) ([]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	names := make([]string, 0, len(e.databases))
	for name := range e.databases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (e *InMemoryEngine) LoadTable(string) (*schema.Table, error) {
	return nil, fmt.Errorf("individual table loading is not supported")
}
func (e *InMemoryEngine) SaveTable(*schema.Table, *transaction.Transaction) error { return nil }
func (e *InMemoryEngine) CreateSnapshot(*schema.Database, string) (uint64, uint32, error) {
	return 0, 0, nil
}
func (e *InMemoryEngine) LoadSnapshot(*schema.Database, string) error { return nil }

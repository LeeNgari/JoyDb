package manager

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
	"github.com/leengari/mini-rdbms/internal/storage/engine"
)

// Registry manages loaded databases in a thread-safe way
type Registry struct {
	mu            sync.RWMutex
	loaded        map[string]*schema.Database
	basePath      string
	storageEngine engine.StorageEngine
}

// NewRegistry creates a new database registry with the given storage engine
func NewRegistry(basePath string, storageEngine engine.StorageEngine) *Registry {
	return &Registry{
		loaded:        make(map[string]*schema.Database),
		basePath:      basePath,
		storageEngine: storageEngine,
	}
}

// Get loads a database (or returns cached one) and ensures indexes are built
func (r *Registry) Get(name string) (*schema.Database, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check cache
	if db, ok := r.loaded[name]; ok {
		return db, nil
	}

	// Load from disk using storage engine
	dbPath := filepath.Join(r.basePath, name)
	db, err := r.storageEngine.LoadDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	// Build Indexes
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		return nil, fmt.Errorf("failed to build indexes: %w", err)
	}

	r.loaded[name] = db
	return db, nil
}

// Create creates a new database
func (r *Registry) Create(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.loaded[name]; ok {
		return fmt.Errorf("database '%s' already exists (loaded)", name)
	}

	return r.storageEngine.CreateDatabase(name, r.basePath)
}

// Drop unloads and deletes a database
func (r *Registry) Drop(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.loaded, name)
	return r.storageEngine.DropDatabase(name, r.basePath)
}

// Rename saves, unloads, and renames a database
func (r *Registry) Rename(oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If loaded, we must unload/save
	if db, ok := r.loaded[oldName]; ok {
		// Create a transaction for the save operation
		tx := transaction.NewTransaction()
		defer tx.Close()

		if err := r.storageEngine.SaveDatabase(db, tx); err != nil {
			return fmt.Errorf("failed to save database before rename: %w", err)
		}
		delete(r.loaded, oldName)
	}

	return r.storageEngine.RenameDatabase(oldName, newName, r.basePath)
}

// SaveAll saves all currently loaded databases
func (r *Registry) SaveAll(tx *transaction.Transaction) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, db := range r.loaded {
		if err := r.storageEngine.SaveDatabase(db, tx); err != nil {
			slog.Error("failed to save database", "name", db.Name, "error", err)
		}
	}
}

// List returns a list of all available databases
func (r *Registry) List() ([]string, error) {
	return r.storageEngine.ListDatabases(r.basePath)
}

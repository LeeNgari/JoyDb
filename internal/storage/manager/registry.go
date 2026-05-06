package manager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
	"github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/wal"
)

// Registry manages loaded databases in a thread-safe way
type Registry struct {
	mu                 sync.RWMutex
	loaded             map[string]*schema.Database
	walManagers        map[string]*WALManager // Per-database WAL managers
	basePath           string
	storageEngine      engine.StorageEngine
	walEnabled         bool          // Whether WAL is enabled globally
	checkpointInterval time.Duration // Interval for auto-checkpoints
}

// NewRegistry creates a new database registry with the given storage engine
func NewRegistry(basePath string, storageEngine engine.StorageEngine) *Registry {
	return NewRegistryWithWAL(basePath, storageEngine, true, 5*time.Second) // WAL enabled by default, 5s checkpoint
}

// NewRegistryWithWAL creates a new database registry with explicit WAL configuration
func NewRegistryWithWAL(
	basePath string,
	storageEngine engine.StorageEngine,
	walEnabled bool,
	checkpointInterval time.Duration,
) *Registry {
	if checkpointInterval == 0 {
		checkpointInterval = 5 * time.Second
	}
	return &Registry{
		loaded:             make(map[string]*schema.Database),
		walManagers:        make(map[string]*WALManager),
		basePath:           basePath,
		storageEngine:      storageEngine,
		walEnabled:         walEnabled,
		checkpointInterval: checkpointInterval,
	}
}

// Get loads a database (or returns cached one) and ensures indexes are built
// Deprecated: Use GetWithWAL for WAL support
func (r *Registry) Get(name string) (*schema.Database, error) {
	db, _, err := r.GetWithWAL(name)
	return db, err
}

// GetWithWAL loads a database with its WAL manager
// If WAL recovery is needed, it will be performed before returning
func (r *Registry) GetWithWAL(name string) (*schema.Database, *WALManager, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check cache
	if db, ok := r.loaded[name]; ok {
		walMgr := r.walManagers[name]
		return db, walMgr, nil
	}

	// Load from disk using storage engine
	dbPath := filepath.Join(r.basePath, name)
	db, err := r.storageEngine.LoadDatabase(dbPath)
	if err != nil {
		return nil, nil, err
	}

	// Build Indexes
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		return nil, nil, fmt.Errorf("failed to build indexes: %w", err)
	}

	// Initialize WAL manager if enabled
	var walMgr *WALManager
	if r.walEnabled {
		// Create save callback for checkpointer
		// We pass nil transaction as this is internal/system save
		saveFunc := func() error {
			return r.storageEngine.SaveDatabase(db, nil)
		}

		walMgr, err = NewWALManager(db, dbPath, name, true, r.checkpointInterval, saveFunc, r.storageEngine)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create WAL manager: %w", err)
		}

		// Check if WAL file exists and needs recovery
		walPath := filepath.Join(dbPath, name+".wal")
		if _, statErr := os.Stat(walPath); statErr == nil {
			// WAL file exists - perform recovery with progress logging
			progressCallback := func(p wal.RecoveryProgress) {
				slog.Info("Recovery progress",
					"database", name,
					"phase", p.Phase,
					"percentage", p.Percentage(),
					"processed", p.ProcessedRecords,
				)
			}

			result, recoverErr := walMgr.RecoverWithProgress(progressCallback)
			if recoverErr != nil {
				walMgr.Close()
				return nil, nil, fmt.Errorf("WAL recovery failed (refusing to start): %w", recoverErr)
			}

			// Replay operations if any
			hasOps := len(result.InsertOps) > 0 || len(result.UpdateOps) > 0 || len(result.DeleteOps) > 0 ||
				len(result.CreateTableOps) > 0 || len(result.DropTableOps) > 0 || len(result.AlterTableOps) > 0

			if result != nil && hasOps {
				slog.Info("WAL: Replaying operations",
					"database", name,
					"inserts", len(result.InsertOps),
					"updates", len(result.UpdateOps),
					"deletes", len(result.DeleteOps),
					"create_tables", len(result.CreateTableOps),
					"drop_tables", len(result.DropTableOps),
					"alter_tables", len(result.AlterTableOps),
				)

				target := NewDatabaseReplayTarget(db)
				if replayErr := result.ReplayAll(target); replayErr != nil {
					walMgr.Close()
					return nil, nil, fmt.Errorf("WAL replay failed (refusing to start): %w", replayErr)
				}

				// Rebuild indexes after replay
				if indexErr := indexing.BuildDatabaseIndexes(db); indexErr != nil {
					walMgr.Close()
					return nil, nil, fmt.Errorf("failed to rebuild indexes after WAL replay: %w", indexErr)
				}

				slog.Info("WAL: Recovery complete", "database", name)
			}
		}

		r.walManagers[name] = walMgr
	}

	r.loaded[name] = db
	return db, walMgr, nil
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

	// Close WAL manager if exists
	if walMgr, ok := r.walManagers[name]; ok {
		walMgr.Close()
		delete(r.walManagers, name)
	}

	delete(r.loaded, name)
	return r.storageEngine.DropDatabase(name, r.basePath)
}

// Rename saves, unloads, and renames a database
func (r *Registry) Rename(oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close old WAL manager if exists
	if walMgr, ok := r.walManagers[oldName]; ok {
		walMgr.Close()
		delete(r.walManagers, oldName)
	}

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

// SaveAll saves all currently loaded databases and writes checkpoints
func (r *Registry) SaveAll(tx *transaction.Transaction) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	errChs := make([]<-chan error, 0)

	for name, db := range r.loaded {
		wm, hasWAL := r.walManagers[name]
		if hasWAL && wm.IsEnabled() {
			// Checkpoint (which includes SaveDatabase)
			errChs = append(errChs, wm.AsyncCheckpoint())
		} else {
			// Just save synchronously
			if err := r.storageEngine.SaveDatabase(db, tx); err != nil {
				slog.Error("failed to save database", "name", name, "error", err)
			}
		}
	}

	// Wait for all async checkpoints
	for _, ch := range errChs {
		if err := <-ch; err != nil {
			slog.Error("SaveAll: checkpoint failed", "error", err)
		}
	}
}

// CloseAll closes all WAL managers (call on shutdown)
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, walMgr := range r.walManagers {
		if err := walMgr.Close(); err != nil {
			slog.Error("failed to close WAL manager", "name", name, "error", err)
		}
	}
	r.walManagers = make(map[string]*WALManager)
}

// List returns a list of all available databases
func (r *Registry) List() ([]string, error) {
	return r.storageEngine.ListDatabases(r.basePath)
}

// IsWALEnabled returns whether WAL is enabled for this registry
func (r *Registry) IsWALEnabled() bool {
	return r.walEnabled
}

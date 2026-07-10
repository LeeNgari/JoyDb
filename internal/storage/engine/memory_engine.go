package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/leengari/joydb/internal/domain/transaction"
)

// MemoryEngine implements StorageEngine using binary snapshot files for persistence.
type MemoryEngine struct{}

// NewMemoryEngine creates a new Memory storage engine
func NewMemoryEngine() *MemoryEngine {
	return &MemoryEngine{}
}

// LoadDatabase loads a database from the latest snapshot file
func (e *MemoryEngine) LoadDatabase(dbPath string) (*schema.Database, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database does not exist at path: %s", dbPath)
	}

	entries, err := os.ReadDir(dbPath)
	if err != nil {
		return nil, err
	}

	var latestSnap string
	var latestLSN int64 = -1

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".snap") {
			lsnStr := strings.TrimSuffix(entry.Name(), ".snap")
			lsn, err := strconv.ParseInt(lsnStr, 10, 64)
			if err == nil && lsn > latestLSN {
				latestLSN = lsn
				latestSnap = entry.Name()
			}
		}
	}

	db := &schema.Database{
		Name:   filepath.Base(dbPath),
		Path:   dbPath,
		Tables: make(map[string]*schema.Table),
	}

	if latestSnap != "" {
		snapPath := filepath.Join(dbPath, latestSnap)
		if err := e.LoadSnapshot(db, snapPath); err != nil {
			return nil, fmt.Errorf("failed to load snapshot %s: %w", snapPath, err)
		}
	}

	return db, nil
}

// SaveDatabase persists the current state for callers that are not using WAL checkpoints.
func (e *MemoryEngine) SaveDatabase(db *schema.Database, tx *transaction.Transaction) error {
	_, _, err := e.CreateSnapshot(db, db.Path)
	return err
}

// CreateDatabase creates a new database directory
func (e *MemoryEngine) CreateDatabase(name, basePath string) error {
	dbPath := filepath.Join(basePath, name)
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		return fmt.Errorf("database '%s' already exists", name)
	}
	return os.MkdirAll(dbPath, 0755)
}

// DropDatabase removes a database directory
func (e *MemoryEngine) DropDatabase(name, basePath string) error {
	dbPath := filepath.Join(basePath, name)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database '%s' does not exist", name)
	}
	return os.RemoveAll(dbPath)
}

// RenameDatabase renames a database directory
func (e *MemoryEngine) RenameDatabase(oldName, newName, basePath string) error {
	oldPath := filepath.Join(basePath, oldName)
	newPath := filepath.Join(basePath, newName)

	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("database '%s' does not exist", oldName)
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		return fmt.Errorf("database '%s' already exists", newName)
	}

	return os.Rename(oldPath, newPath)
}

// ListDatabases returns all available databases
func (e *MemoryEngine) ListDatabases(basePath string) ([]string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var databases []string
	for _, entry := range entries {
		if entry.IsDir() {
			databases = append(databases, entry.Name())
		}
	}
	return databases, nil
}

// LoadTable is not supported in MemoryEngine
func (e *MemoryEngine) LoadTable(tablePath string) (*schema.Table, error) {
	return nil, fmt.Errorf("LoadTable not supported in MemoryEngine")
}

// SaveTable is a no-op in MemoryEngine
func (e *MemoryEngine) SaveTable(table *schema.Table, tx *transaction.Transaction) error {
	return nil
}

// CreateSnapshot atomically persists the current in-memory state
func (e *MemoryEngine) CreateSnapshot(db *schema.Database, snapshotDir string) (uint64, uint32, error) {
	return CreateSnapshot(db, snapshotDir)
}

// LoadSnapshot loads a snapshot file into the database
func (e *MemoryEngine) LoadSnapshot(db *schema.Database, snapshotPath string) error {
	return LoadSnapshot(db, snapshotPath)
}

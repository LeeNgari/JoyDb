package engine

import (
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
)

// StorageEngine defines the interface for all storage backends
// This abstraction allows swapping between different storage formats (JSON, Binary, etc.)
type StorageEngine interface {
	// Database Lifecycle Operations

	// LoadDatabase loads a database from the given path
	LoadDatabase(dbPath string) (*schema.Database, error)

	// SaveDatabase persists a database to disk
	SaveDatabase(db *schema.Database, tx *transaction.Transaction) error

	// CreateDatabase creates a new database at the given path
	CreateDatabase(name, basePath string) error

	// DropDatabase removes a database from disk
	DropDatabase(name, basePath string) error

	// RenameDatabase renames a database
	RenameDatabase(oldName, newName, basePath string) error

	// ListDatabases returns all available databases in the base path
	ListDatabases(basePath string) ([]string, error)

	// Table Operations

	// LoadTable loads a single table from the given path
	LoadTable(tablePath string) (*schema.Table, error)

	// SaveTable persists a single table to disk
	SaveTable(table *schema.Table, tx *transaction.Transaction) error
}

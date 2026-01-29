package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/storage/loader"
	"github.com/leengari/mini-rdbms/internal/storage/metadata"
	"github.com/leengari/mini-rdbms/internal/storage/writer"
)

// JSONEngine implements StorageEngine using JSON files for persistence
type JSONEngine struct{}

// NewJSONEngine creates a new JSON storage engine
func NewJSONEngine() *JSONEngine {
	return &JSONEngine{}
}

// LoadDatabase loads a database from JSON files
func (e *JSONEngine) LoadDatabase(dbPath string) (*schema.Database, error) {
	return loader.LoadDatabase(dbPath)
}

// SaveDatabase saves a database to JSON files
func (e *JSONEngine) SaveDatabase(db *schema.Database, tx *transaction.Transaction) error {
	return writer.SaveDatabase(db, tx)
}

// CreateDatabase creates a new database directory with JSON metadata
func (e *JSONEngine) CreateDatabase(name, basePath string) error {
	dbPath := filepath.Join(basePath, name)

	// Check if exists
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		return fmt.Errorf("database '%s' already exists", name)
	}

	// Create directory
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Create meta.json
	meta := metadata.DatabaseMeta{
		Name:    name,
		Version: 1,
		Tables:  []string{},
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metaPath := filepath.Join(dbPath, "meta.json")
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write meta.json: %w", err)
	}

	return nil
}

// DropDatabase removes a database directory
func (e *JSONEngine) DropDatabase(name, basePath string) error {
	dbPath := filepath.Join(basePath, name)

	// Check if exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database '%s' does not exist", name)
	}

	// Remove directory
	if err := os.RemoveAll(dbPath); err != nil {
		return fmt.Errorf("failed to remove database directory: %w", err)
	}

	return nil
}

// RenameDatabase renames a database directory and updates JSON metadata
func (e *JSONEngine) RenameDatabase(oldName, newName, basePath string) error {
	oldPath := filepath.Join(basePath, oldName)
	newPath := filepath.Join(basePath, newName)

	// Check if old exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("database '%s' does not exist", oldName)
	}

	// Check if new exists
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		return fmt.Errorf("database '%s' already exists", newName)
	}

	// Rename directory
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to rename database directory: %w", err)
	}

	// Update meta.json
	metaPath := filepath.Join(newPath, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("failed to read meta.json: %w", err)
	}

	var meta metadata.DatabaseMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("failed to parse meta.json: %w", err)
	}

	meta.Name = newName
	newData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write meta.json: %w", err)
	}

	return nil
}

// ListDatabases returns all available databases
func (e *JSONEngine) ListDatabases(basePath string) ([]string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read databases directory: %w", err)
	}

	var databases []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a valid database (has meta.json)
		metaPath := filepath.Join(basePath, entry.Name(), "meta.json")
		if _, err := os.Stat(metaPath); err == nil {
			databases = append(databases, entry.Name())
		}
	}

	return databases, nil
}

// LoadTable loads a single table from JSON files
func (e *JSONEngine) LoadTable(tablePath string) (*schema.Table, error) {
	return loader.LoadTable(tablePath)
}

// SaveTable saves a single table to JSON files
func (e *JSONEngine) SaveTable(table *schema.Table, tx *transaction.Transaction) error {
	return writer.SaveTable(table, tx)
}

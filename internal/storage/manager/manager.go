package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leengari/mini-rdbms/internal/storage/metadata"
)

// CreateDatabase creates a new database directory and meta.json
func CreateDatabase(name string, basePath string) error {
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
func DropDatabase(name string, basePath string) error {
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

// RenameDatabase renames a database directory and updates meta.json
func RenameDatabase(oldName, newName string, basePath string) error {
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

// ListDatabases returns a list of all available databases in the base path
func ListDatabases(basePath string) ([]string, error) {
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

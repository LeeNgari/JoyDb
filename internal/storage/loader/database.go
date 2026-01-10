package loader

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/storage/metadata"
)

// LoadDatabase loads the database from the given directory path
func LoadDatabase(dbPath string) (*schema.Database, error) {
	metaPath := filepath.Join(dbPath, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read database meta: %w", err)
	}

	var meta metadata.DatabaseMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse database meta: %w", err)
	}

	db := &schema.Database{
		Name:   meta.Name,
		Path:   dbPath,
		Tables: make(map[string]*schema.Table),
	}

	// Read all entries in the database directory
	entries, err := os.ReadDir(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read database directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		tableName := entry.Name()
		tablePath := filepath.Join(dbPath, tableName)

		table, err := LoadTable(tablePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load table %s: %w", tableName, err)
		}

		db.Tables[table.Name] = table
	}

	slog.Info("Database loaded successfully",
		slog.String("name", db.Name),
		slog.String("path", dbPath),
		slog.Int("table_count", len(db.Tables)),
	)

	return db, nil
}

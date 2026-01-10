package writer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/storage/metadata"
)

// SaveTable persists both data.json and meta.json atomically
func SaveTable(t *schema.Table) error {
	if t == nil || t.Path == "" {
		return fmt.Errorf("cannot save table: nil or missing path")
	}

	tableName := t.Name
	basePath := t.Path

	// Lock table for reading during save
	t.RLock()
	defer t.RUnlock()

	// 1. Prepare meta (updated from current in-memory state)
	meta := metadata.TableMeta{
		Name:         tableName,
		LastInsertID: t.LastInsertID,
		RowCount:     int64(len(t.Rows)), // Calculate from actual rows
		Columns:      make([]metadata.ColumnMeta, len(t.Schema.Columns)),
	}

	for i, col := range t.Schema.Columns {
		meta.Columns[i] = metadata.ColumnMeta{
			Name:          col.Name,
			Type:          string(col.Type),
			PrimaryKey:    col.PrimaryKey,
			Unique:        col.Unique,
			NotNull:       col.NotNull,
			AutoIncrement: col.AutoIncrement,
		}
	}

	// 2. Marshal meta
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal table meta for %s: %w", tableName, err)
	}

	// 3. Marshal data (rows)
	dataBytes, err := json.MarshalIndent(t.Rows, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rows for %s: %w", tableName, err)
	}

	// 4. Write both files using temp + atomic rename
	files := []struct {
		path string
		data []byte
		name string
	}{
		{filepath.Join(basePath, "meta.json"), metaBytes, "meta.json"},
		{filepath.Join(basePath, "data.json"), dataBytes, "data.json"},
	}

	for _, f := range files {
		tmpPath := f.path + ".tmp"

		// Write to temp
		if err := os.WriteFile(tmpPath, f.data, 0644); err != nil {
			return fmt.Errorf("failed to write temp file %s for table %s: %w", f.name, tableName, err)
		}

		// Atomic replace
		if err := os.Rename(tmpPath, f.path); err != nil {
			return fmt.Errorf("failed to rename temp → %s for table %s: %w", f.name, tableName, err)
		}
	}

	slog.Info("Table saved successfully",
		slog.String("table", tableName),
		slog.String("path", basePath),
		slog.Int64("last_insert_id", t.LastInsertID),
		slog.Int("row_count", len(t.Rows)),
	)

	return nil
}

// SaveDatabase saves all tables and database metadata
func SaveDatabase(db *schema.Database) error {
	if db == nil {
		return fmt.Errorf("cannot save nil database")
	}

	// 1. Save all tables first
	for name, table := range db.Tables {
		if err := SaveTable(table); err != nil {
			slog.Error("failed to save table during database save",
				slog.String("table", name),
				slog.Any("error", err),
			)
			return fmt.Errorf("failed to save table %s: %w", name, err)
		}
	}

	// 2. Build table list from current state
	tableNames := make([]string, 0, len(db.Tables))
	for name := range db.Tables {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames) // Deterministic order

	// 3. Create database metadata
	dbMeta := metadata.DatabaseMeta{
		Name:    db.Name,
		Version: 1, // Could be incremented for schema migrations
		Tables:  tableNames,
	}

	// 4. Marshal database metadata
	metaBytes, err := json.MarshalIndent(dbMeta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal database meta: %w", err)
	}

	// 5. Save database meta.json atomically
	dbMetaPath := filepath.Join(db.Path, "meta.json")
	tmpPath := dbMetaPath + ".tmp"

	if err := os.WriteFile(tmpPath, metaBytes, 0644); err != nil {
		return fmt.Errorf("failed to write temp database meta: %w", err)
	}

	if err := os.Rename(tmpPath, dbMetaPath); err != nil {
		return fmt.Errorf("failed to rename temp → database meta.json: %w", err)
	}

	slog.Info("Database saved successfully",
		slog.String("name", db.Name),
		slog.String("path", db.Path),
		slog.Int("table_count", len(db.Tables)),
	)

	return nil
}

// FlushTableIfDirty saves the table only if it has unsaved changes
func FlushTableIfDirty(table *schema.Table) error {
	// Check dirty flag (needs lock)
	table.RLock()
	isDirty := table.Dirty
	table.RUnlock()

	if !isDirty {
		return nil
	}

	if err := SaveTable(table); err != nil {
		return err
	}

	// Clear dirty flag
	table.Lock()
	table.Dirty = false
	table.Unlock()

	return nil
}
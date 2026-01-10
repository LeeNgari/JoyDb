package loader

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/query/validation"
	"github.com/leengari/mini-rdbms/internal/storage/metadata"
)

// LoadTable loads a table from the given directory path
func LoadTable(path string) (*schema.Table, error) {
	metaPath := filepath.Join(path, "meta.json")
	dataPath := filepath.Join(path, "data.json")

	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta metadata.TableMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, err
	}

	tableSchema := &schema.TableSchema{
		TableName: meta.Name,
		Columns:   make([]schema.Column, 0),
	}

	for _, c := range meta.Columns {
		col := schema.Column{
			Name:          c.Name,
			Type:          schema.ColumnType(c.Type),
			PrimaryKey:    c.PrimaryKey,
			Unique:        c.Unique,
			NotNull:       c.NotNull,
			AutoIncrement: c.AutoIncrement,
		}
		tableSchema.Columns = append(tableSchema.Columns, col)
	}

	rows := []data.Row{}
	if _, err := os.Stat(dataPath); err == nil {
		dataBytes, err := os.ReadFile(dataPath)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(dataBytes, &rows); err != nil {
			return nil, err
		}
	}

	table := &schema.Table{
		Name:         meta.Name,
		Path:         path,
		Schema:       tableSchema,
		Rows:         rows,
		Indexes:      make(map[string]*data.Index),
		LastInsertID: meta.LastInsertID,
	}

	// Validate all loaded rows against schema
	for i, row := range table.Rows {
		if err := validation.ValidateRow(table, row, i); err != nil {
			return nil, fmt.Errorf("data validation failed for row %d in table %s: %w", i, meta.Name, err)
		}
	}

	slog.Info("table loaded",
		slog.String("table", table.Name),
		slog.Int("rows", len(rows)),
		slog.String("path", path),
	)

	return table, nil
}

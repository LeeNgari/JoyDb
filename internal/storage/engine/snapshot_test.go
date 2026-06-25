package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/index/btree"
)

func TestSnapshotRoundtrip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "joydb_snapshot_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbName := "testdb"
	dbPath := filepath.Join(tempDir, dbName)

	db := &schema.Database{
		Name:   dbName,
		Path:   dbPath,
		Tables: make(map[string]*schema.Table),
	}

	tableSchema := &schema.TableSchema{
		TableName: "users",
		Columns: []schema.Column{
			{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
			{Name: "name", Type: schema.ColumnTypeText},
			{Name: "age", Type: schema.ColumnTypeFloat},
			{Name: "active", Type: schema.ColumnTypeBool},
		},
	}

	table := &schema.Table{
		Name:         "users",
		Path:         filepath.Join(dbPath, "users"),
		Schema:       tableSchema,
		RowsByRID:    make(map[int64]data.Row),
		PKIndex:      btree.New(32),
		Indexes:      make(map[string]*data.Index),
		LastInsertID: 42,
	}

	// Add some rows
	row1 := data.NewRow(map[string]interface{}{
		"id":     int64(1),
		"name":   "Alice",
		"age":    30.5,
		"active": true,
	})
	table.InsertReplay(row1)

	row2 := data.NewRow(map[string]interface{}{
		"id":     int64(2),
		"name":   "Bob",
		"age":    float64(25),
		"active": false,
	})
	table.InsertReplay(row2)

	db.Tables["users"] = table

	// Create Snapshot
	lsn, crc, err := CreateSnapshot(db, dbPath)
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	if lsn == 0 || crc == 0 {
		t.Fatalf("Invalid LSN or CRC returned: LSN=%d, CRC=%d", lsn, crc)
	}

	// Load Snapshot into a new database object
	loadedDB := &schema.Database{
		Name:   dbName,
		Path:   dbPath,
		Tables: make(map[string]*schema.Table),
	}


	// Real path
	snapFile := ""
	entries, _ := os.ReadDir(dbPath)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".snap" {
			snapFile = filepath.Join(dbPath, e.Name())
			break
		}
	}

	if err := LoadSnapshot(loadedDB, snapFile); err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	// Verify
	loadedTable, ok := loadedDB.Tables["users"]
	if !ok {
		t.Fatalf("Table 'users' not found in loaded snapshot")
	}

	if loadedTable.LastInsertID != 42 {
		t.Errorf("Expected LastInsertID 42, got %d", loadedTable.LastInsertID)
	}

	if loadedTable.LiveRowCount() != 2 {
		t.Fatalf("Expected 2 rows, got %d", loadedTable.LiveRowCount())
	}

	r1 := loadedTable.LiveRows()[0]
	if r1.Data["id"] != int64(1) || r1.Data["name"] != "Alice" || r1.Data["age"] != 30.5 || r1.Data["active"] != true {
		t.Errorf("Row 1 data mismatch: %v", r1.Data)
	}
}

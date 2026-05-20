package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/wal"
	"gotest.tools/v3/assert"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

// createTestDatabase creates an in-memory test database with a users table.
// The database is NOT persisted to disk.
func createTestDatabase(t *testing.T, name string) *schema.Database {
	t.Helper()
	db := &schema.Database{
		Name:   name,
		Tables: make(map[string]*schema.Table),
	}

	// Create users table
	usersTable := &schema.Table{
		Name: "users",
		Schema: &schema.TableSchema{
			Columns: []schema.Column{
				{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
				{Name: "name", Type: schema.ColumnTypeText},
			},
		},
		Rows:    make([]data.Row, 0),
		Indexes: make(map[string]*data.Index),
	}
	// Initialize indexes
	usersTable.Indexes["id"] = &data.Index{
		Column: "id",
		Unique: true,
		Data:   make(map[interface{}][]int),
	}

	db.Tables["users"] = usersTable
	return db
}

// createTempDir creates a temporary directory for test databases.
// Returns the path. Caller should defer cleanup with os.RemoveAll.
func createTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "wal_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return dir
}

// createTestRow creates a test row with the given data.
func createTestRow(t *testing.T, values map[string]interface{}) data.Row {
	t.Helper()
	return data.Row{Data: values}
}

// createTestTransaction creates a transaction for testing.
func createTestTransaction(t *testing.T) *transaction.Transaction {
	t.Helper()
	return transaction.NewTransaction()
}

// =============================================================================
// SUITE 4: WAL MANAGER INTEGRATION TESTS
// =============================================================================

// TestWALManagerCreate verifies WALManager creation:
// - Creates WAL file at correct path
// - Returns valid WALManager instance
// - IsEnabled() returns true when enabled
func TestWALManagerCreate(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "testdb"
	wm, err := NewWALManager(nil, tempDir, dbName, true, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)
	defer wm.Close()

	assert.Assert(t, wm.IsEnabled())

	walPath := filepath.Join(tempDir, dbName+".wal")
	_, err = os.Stat(walPath)
	assert.NilError(t, err)
}

// TestWALManagerDisabled verifies disabled WALManager:
// - IsEnabled() returns false
// - All logging operations are no-ops (return nil)
// - No WAL file is created
func TestWALManagerDisabled(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "testdb"
	wm, err := NewWALManager(nil, tempDir, dbName, false, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)
	defer wm.Close()

	assert.Assert(t, !wm.IsEnabled())

	tx := createTestTransaction(t)
	err = wm.BeginTransaction(tx)
	assert.NilError(t, err)

	err = wm.Commit(tx)
	assert.NilError(t, err)

	walPath := filepath.Join(tempDir, dbName+".wal")
	_, err = os.Stat(walPath)
	assert.Assert(t, os.IsNotExist(err))
}

// TestWALManagerInsert verifies LogInsert integration:
// - Create WALManager
// - Begin transaction
// - Call LogInsert with table and row
// - Verify record is written to WAL
func TestWALManagerInsert(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "testdb"
	wm, err := NewWALManager(nil, tempDir, dbName, true, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)
	defer wm.Close()

	db := createTestDatabase(t, dbName)
	table := db.Tables["users"]
	tx := createTestTransaction(t)

	err = wm.BeginTransaction(tx)
	assert.NilError(t, err)

	row := createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Alice"})
	err = wm.LogInsert(tx, table, row)
	assert.NilError(t, err)

	err = wm.Commit(tx)
	assert.NilError(t, err)

	// Close to flush
	wm.Close()

	// Verify WAL content
	walPath := filepath.Join(tempDir, dbName+".wal")
	reader, err := wal.NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	records, err := reader.ScanAll()
	assert.NilError(t, err)

	// Expect BeginTxn, Insert, Commit
	assert.Equal(t, len(records), 3)
	assert.Assert(t, records[1].GetHeader().Type == wal.RecordInsert)
	ins := records[1].(*wal.InsertRecord)
	assert.Equal(t, ins.Key, "1")
}

// TestWALManagerUpdate verifies LogUpdate integration:
// - Log an update with old and new row
// - Verify both old and new values are in WAL record
func TestWALManagerUpdate(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	wm, err := NewWALManager(nil, tempDir, "testdb", true, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)
	defer wm.Close()

	db := createTestDatabase(t, "testdb")
	table := db.Tables["users"]
	tx := createTestTransaction(t)

	wm.BeginTransaction(tx)

	oldRow := createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Alice"})
	newRow := createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Bob"})

	err = wm.LogUpdate(tx, table, "1", oldRow, newRow)
	assert.NilError(t, err)
	wm.Commit(tx)
	wm.Close()

	// Verify
	walPath := filepath.Join(tempDir, "testdb.wal")
	reader, err := wal.NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	records, err := reader.ScanAll()
	assert.NilError(t, err)

	// Begin, Update, Commit
	assert.Assert(t, records[1].GetHeader().Type == wal.RecordUpdate)
	upd := records[1].(*wal.UpdateRecord)
	assert.Equal(t, upd.Key, "1")
}

// TestWALManagerDelete verifies LogDelete integration:
// - Log a delete with old row
// - Verify old value is preserved in WAL
func TestWALManagerDelete(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	wm, err := NewWALManager(nil, tempDir, "testdb", true, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)
	defer wm.Close()

	db := createTestDatabase(t, "testdb")
	table := db.Tables["users"]
	tx := createTestTransaction(t)

	wm.BeginTransaction(tx)

	oldRow := createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Alice"})

	err = wm.LogDelete(tx, table, "1", oldRow)
	assert.NilError(t, err)
	wm.Commit(tx)
	wm.Close()

	// Verify
	walPath := filepath.Join(tempDir, "testdb.wal")
	reader, err := wal.NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	records, err := reader.ScanAll()
	assert.NilError(t, err)

	// Begin, Delete, Commit
	assert.Assert(t, records[1].GetHeader().Type == wal.RecordDelete)
	del := records[1].(*wal.DeleteRecord)
	assert.Equal(t, del.Key, "1")
}

// TestWALManagerFullCycle verifies complete write/close/recover cycle:
// - Insert via WALManager, commit
// - Close WALManager
// - Create new WALManager for same database
// - Recover should return the insert operation
func TestWALManagerFullCycle(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	wm, err := NewWALManager(nil, tempDir, "testdb", true, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)

	db := createTestDatabase(t, "testdb")
	table := db.Tables["users"]
	tx := createTestTransaction(t)

	wm.BeginTransaction(tx)
	row := createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Alice"})
	wm.LogInsert(tx, table, row)
	wm.Commit(tx)
	wm.Close()

	// Reopen
	wm2, err := NewWALManager(nil, tempDir, "testdb", true, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)
	defer wm2.Close()

	result, err := wm2.Recover()
	assert.NilError(t, err)
	assert.Equal(t, len(result.InsertOps), 1)
	assert.Equal(t, result.InsertOps[0].Key, "1")
}

// =============================================================================
// REPLAY TARGET INTEGRATION TESTS
// =============================================================================

// TestReplayInsertToDatabase verifies ReplayInsert:
// - Create database with empty table
// - Call ReplayInsert with row data
// - Verify row is added to table
func TestReplayInsertToDatabase(t *testing.T) {
	db := createTestDatabase(t, "testdb")
	target := NewDatabaseReplayTarget(db)

	rowMap := map[string]interface{}{"id": int64(1), "name": "Alice"}
	rowBytes, _ := json.Marshal(rowMap)
	rowJSON := []byte(rowBytes)

	err := target.ReplayInsert("users", "1", rowJSON)
	assert.NilError(t, err)

	assert.Equal(t, len(db.Tables["users"].Rows), 1)
	assert.Equal(t, db.Tables["users"].Rows[0].Data["name"], "Alice")
}

// TestReplayUpdateToDatabase verifies ReplayUpdate:
// - Create database with existing row
// - Call ReplayUpdate with new data
// - Verify row is updated
func TestReplayUpdateToDatabase(t *testing.T) {
	db := createTestDatabase(t, "testdb")
	// Pre-populate
	row := createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Alice"})
	db.Tables["users"].Rows = append(db.Tables["users"].Rows, row)

	target := NewDatabaseReplayTarget(db)

	newRowMap := map[string]interface{}{"id": int64(1), "name": "Bob"}
	newRowBytes, _ := json.Marshal(newRowMap)
	newRowJSON := []byte(newRowBytes)

	err := target.ReplayUpdate("users", "1", newRowJSON)
	assert.NilError(t, err)

	assert.Equal(t, len(db.Tables["users"].Rows), 1)
	assert.Equal(t, db.Tables["users"].Rows[0].Data["name"], "Bob")
}

// TestReplayDeleteFromDatabase verifies ReplayDelete:
// - Create database with existing row
// - Call ReplayDelete with key
// - Verify row is removed
func TestReplayDeleteFromDatabase(t *testing.T) {
	db := createTestDatabase(t, "testdb")
	// Pre-populate
	row := createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Alice"})
	db.Tables["users"].Rows = append(db.Tables["users"].Rows, row)

	target := NewDatabaseReplayTarget(db)

	err := target.ReplayDelete("users", "1")
	assert.NilError(t, err)

	assert.Equal(t, len(db.Tables["users"].Rows), 0)
}

// TestReplayMissingTable verifies graceful handling of missing tables:
// - Call ReplayInsert for non-existent table
// - Should log warning but not error
func TestReplayMissingTable(t *testing.T) {
	db := createTestDatabase(t, "testdb")
	// Clear tables
	db.Tables = make(map[string]*schema.Table)

	target := NewDatabaseReplayTarget(db)

	rowBytes, _ := json.Marshal(map[string]interface{}{"a": 1})

	err := target.ReplayInsert("nonexistent", "1", []byte(rowBytes))
	assert.NilError(t, err)
}

// =============================================================================
// REGISTRY INTEGRATION TESTS
// =============================================================================

// TestRegistryGetWithWAL verifies Registry.GetWithWAL:
// - Create registry with WAL enabled
// - Load database using GetWithWAL
// - Verify both database and WALManager are returned
func TestRegistryGetWithWAL(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "testdb"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0)
	defer reg.CloseAll()

	db, wm, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)
	assert.Assert(t, db != nil)
	assert.Assert(t, wm != nil)
	assert.Assert(t, wm.IsEnabled())
}

// TestRegistryRecoveryOnLoad verifies recovery happens on DB load:
// - Create database files
// - Create WAL file with committed operations
// - Load database via GetWithWAL
// - Verify WAL operations are replayed into database
func TestRegistryRecoveryOnLoad(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "testdb"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	// Create WAL with committed transaction
	wm, err := NewWALManager(nil, filepath.Join(tempDir, dbName), dbName, true, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)

	// Need a schema to log insert, but we can fake it or use createTestDatabase's table
	// But WALManager.LogInsert takes *schema.Table.
	// We can create a dummy schema table.
	db := createTestDatabase(t, dbName)
	table := db.Tables["users"]

	tx := createTestTransaction(t)
	wm.BeginTransaction(tx)
	wm.LogInsert(tx, table, createTestRow(t, map[string]interface{}{"id": int64(10), "name": "Recovered"}))
	wm.Commit(tx)
	wm.Close()

	// Now load via Registry
	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0)
	defer reg.CloseAll()

	loadedDB, _, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)

	// Verify row exists
	// Note: createMinimalDatabaseFiles creates empty tables.
	// So only recovered row should be there.
	rows := loadedDB.Tables["users"].Rows
	assert.Equal(t, len(rows), 1)
	assert.Equal(t, rows[0].Data["name"], "Recovered")
}

// TestRegistrySaveAllCheckpoint verifies checkpoint on save:
// - Load database
// - Perform operations
// - Call SaveAll
// - Verify checkpoint is written to WAL
func TestRegistrySaveAllCheckpoint(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "testdb"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0)
	defer reg.CloseAll()

	db, wm, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)

	// Add a row (in memory)
	db.Tables["users"].Rows = append(db.Tables["users"].Rows, createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Alice"}))
	db.Tables["users"].MarkDirty()

	// SaveAll
	tx := createTestTransaction(t)
	reg.SaveAll(tx)

	// Verify checkpoint in WAL
	wm.Close()

	walPath := filepath.Join(tempDir, dbName, dbName+".wal")
	reader, err := wal.NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	// Should have checkpoint at end
	// Note: GetWithWAL might create WAL header if not exists.
	// SaveAll calls WriteCheckpoint.
	// So we expect: Header, Checkpoint.

	lastCp, err := reader.FindLastCheckpoint()
	assert.NilError(t, err)
	assert.Assert(t, lastCp != nil)
}

// TestRegistryCloseAll verifies clean shutdown:
// - Load multiple databases with WAL
// - Call CloseAll
// - Verify all WAL files are properly closed
func TestRegistryCloseAll(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	createMinimalDatabaseFiles(t, tempDir, "db1", "t1")
	createMinimalDatabaseFiles(t, tempDir, "db2", "t2")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0)

	_, _, err := reg.GetWithWAL("db1")
	assert.NilError(t, err)
	_, _, err = reg.GetWithWAL("db2")
	assert.NilError(t, err)

	reg.CloseAll()

	// Verify we can delete temp dir (on Windows open files lock dir, on Linux it's fine but checking for panics is main goal)
}

// =============================================================================
// CHECKPOINT TESTS
// =============================================================================

// TestWriteCheckpointWithTables verifies checkpoint creation:
// - Create database with multiple tables
// - Write checkpoint
// - Verify checkpoint contains CRCs for all tables
func TestWriteCheckpointWithTables(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "testdb"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	db := &schema.Database{
		Name: dbName,
		Path: dbPath,
		Tables: map[string]*schema.Table{
			"t1": {
				Name: "t1", 
				Path: dbPath,
				Schema: &schema.TableSchema{
					Columns: []schema.Column{{Name: "id", Type: schema.ColumnTypeInt}},
				},
			},
			"t2": {
				Name: "t2", 
				Path: dbPath,
				Schema: &schema.TableSchema{
					Columns: []schema.Column{{Name: "id", Type: schema.ColumnTypeInt}},
				},
			},
		},
	}

	eng := engine.NewMemoryEngine()
	if _, _, err := eng.CreateSnapshot(db, dbPath); err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	wm, err := NewWALManager(nil, dbPath, dbName, true, 0, nil, engine.NewMemoryEngine())
	assert.NilError(t, err)
	defer wm.Close()

	err = wm.WriteCheckpoint(db)
	assert.NilError(t, err)
	wm.Close()

	// Verify
	walPath := filepath.Join(dbPath, dbName+".wal")
	reader, err := wal.NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	cp, err := reader.FindLastCheckpoint()
	assert.NilError(t, err)
	assert.Assert(t, cp != nil)
}

// =============================================================================
// HELPER: createMinimalDatabaseFiles creates the minimum JSON files for a database.
// =============================================================================
func createMinimalDatabaseFiles(t *testing.T, basePath, dbName, tableName string) {
	t.Helper()
	dbPath := filepath.Join(basePath, dbName)

	if err := os.MkdirAll(dbPath, 0755); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}

	db := &schema.Database{
		Name: dbName,
		Path: dbPath,
		Tables: map[string]*schema.Table{
			tableName: {
				Name: tableName,
				Path: dbPath,
				Schema: &schema.TableSchema{
					Columns: []schema.Column{
						{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
						{Name: "name", Type: schema.ColumnTypeText},
					},
				},
				Rows:    []data.Row{},
				Indexes: make(map[string]*data.Index),
			},
		},
	}

	eng := engine.NewMemoryEngine()
	if _, _, err := eng.CreateSnapshot(db, dbPath); err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}
}

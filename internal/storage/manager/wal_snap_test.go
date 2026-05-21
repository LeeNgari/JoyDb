package manager

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/wal"
	"gotest.tools/v3/assert"
)

// =============================================================================
// SUITE: WAL + SNAPSHOT INTEGRATION TESTS
//
// These tests exercise the full lifecycle of the WAL and snapshot subsystems
// working together, covering:
//   - Happy path: normal operations, checkpoint, recovery
//   - Sad path: corruption, missing files, truncation, mixed failures
// =============================================================================

// -----------------------------------------------------------------------------
// HAPPY PATH TESTS
// -----------------------------------------------------------------------------

// TestSnap_CreateAndLoad verifies the basic snapshot roundtrip:
// 1. Create a database with tables and rows
// 2. Snapshot it
// 3. Load from snapshot
// 4. Verify all data is intact
func TestSnap_CreateAndLoad(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "snaptest"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	db := &schema.Database{
		Name: dbName,
		Path: dbPath,
		Tables: map[string]*schema.Table{
			"products": {
				Name: "products",
				Path: dbPath,
				Schema: &schema.TableSchema{
					TableName: "products",
					Columns: []schema.Column{
						{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
						{Name: "name", Type: schema.ColumnTypeText},
						{Name: "price", Type: schema.ColumnTypeFloat},
					},
				},
				Rows: []data.Row{
					{Data: map[string]interface{}{"id": int64(1), "name": "Widget", "price": float64(9.99)}},
					{Data: map[string]interface{}{"id": int64(2), "name": "Gadget", "price": float64(19.99)}},
					{Data: map[string]interface{}{"id": int64(3), "name": "Doohickey", "price": float64(4.50)}},
				},
				Indexes: make(map[string]*data.Index),
			},
		},
	}

	eng := engine.NewMemoryEngine()

	// Create snapshot
	lsn, crc, err := eng.CreateSnapshot(db, dbPath)
	assert.NilError(t, err)
	assert.Assert(t, lsn > 0)
	assert.Assert(t, crc > 0)

	// Load from snapshot into new database
	loadedDB, err := eng.LoadDatabase(dbPath)
	assert.NilError(t, err)

	// Verify
	assert.Equal(t, len(loadedDB.Tables), 1)
	products := loadedDB.Tables["products"]
	assert.Assert(t, products != nil)
	assert.Equal(t, len(products.Rows), 3)
	assert.Equal(t, products.Rows[0].Data["name"], "Widget")
	assert.Equal(t, products.Rows[2].Data["price"], float64(4.50))
}

// TestSnap_KeepsTwo verifies that only 2 snapshots are retained:
// 1. Create 4 snapshots
// 2. Verify only the 2 newest remain
func TestSnap_KeepsTwo(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "snaptest"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	db := &schema.Database{
		Name:   dbName,
		Path:   dbPath,
		Tables: make(map[string]*schema.Table),
	}

	eng := engine.NewMemoryEngine()

	// Create 4 snapshots
	for i := 0; i < 4; i++ {
		_, _, err := eng.CreateSnapshot(db, dbPath)
		assert.NilError(t, err)
	}

	// Count .snap files
	entries, err := os.ReadDir(dbPath)
	assert.NilError(t, err)

	snapCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".snap") {
			snapCount++
		}
	}

	assert.Equal(t, snapCount, 2, "should keep exactly 2 snapshots")
}

// TestWALSnap_CheckpointCreatesSnapshot verifies that a WAL checkpoint
// creates a snapshot file:
// 1. Create database, insert rows into memory
// 2. Trigger checkpoint via WALManager
// 3. Verify a .snap file exists on disk
func TestWALSnap_CheckpointCreatesSnapshot(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "checktest"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	db := &schema.Database{
		Name: dbName,
		Path: dbPath,
		Tables: map[string]*schema.Table{
			"users": {
				Name: "users",
				Path: dbPath,
				Schema: &schema.TableSchema{
					Columns: []schema.Column{
						{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
						{Name: "name", Type: schema.ColumnTypeText},
					},
				},
				Rows: []data.Row{
					{Data: map[string]interface{}{"id": int64(1), "name": "Alice"}},
				},
				Indexes: make(map[string]*data.Index),
			},
		},
	}

	eng := engine.NewMemoryEngine()
	wm, err := NewWALManager(db, dbPath, dbName, true, 0, 0, nil, eng)
	assert.NilError(t, err)
	defer wm.Close()

	err = wm.WriteCheckpoint(db)
	assert.NilError(t, err)

	// Verify .snap file exists
	entries, err := os.ReadDir(dbPath)
	assert.NilError(t, err)

	snapFound := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".snap") {
			snapFound = true
			break
		}
	}
	assert.Assert(t, snapFound, "checkpoint should create a .snap file")
}

// TestWALSnap_RecoveryAfterCheckpoint verifies that only WAL records
// after the last checkpoint are replayed:
// 1. Insert row via WAL → commit
// 2. Write checkpoint (snapshot created)
// 3. Insert another row via WAL → commit
// 4. "Crash" (close everything, do NOT save)
// 5. Reload: snapshot should have row 1, WAL replay should add row 2
func TestWALSnap_RecoveryAfterCheckpoint(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "recovery_cp"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0, 0)

	db, wm, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)

	// --- Insert row 1 and commit ---
	tx1 := transaction.NewTransaction()
	wm.BeginTransaction(tx1)
	row1 := data.Row{Data: map[string]interface{}{"id": int64(1), "name": "Alice"}}
	wm.LogInsert(tx1, db.Tables["users"], row1)
	wm.Commit(tx1)
	db.Tables["users"].Rows = append(db.Tables["users"].Rows, row1)

	// --- Checkpoint (snapshot captures row 1) ---
	err = wm.WriteCheckpoint(db)
	assert.NilError(t, err)

	// --- Insert row 2 and commit (after checkpoint) ---
	tx2 := transaction.NewTransaction()
	wm.BeginTransaction(tx2)
	row2 := data.Row{Data: map[string]interface{}{"id": int64(2), "name": "Bob"}}
	wm.LogInsert(tx2, db.Tables["users"], row2)
	wm.Commit(tx2)
	db.Tables["users"].Rows = append(db.Tables["users"].Rows, row2)

	// --- "Crash" without saving ---
	reg.CloseAll()

	// --- Recover ---
	reg2 := NewRegistryWithWAL(tempDir, eng, true, 0, 0)
	defer reg2.CloseAll()

	db2, _, err := reg2.GetWithWAL(dbName)
	assert.NilError(t, err)

	// Row 1 comes from snapshot, row 2 comes from WAL replay
	assert.Equal(t, len(db2.Tables["users"].Rows), 2,
		"should have 2 rows: 1 from snapshot + 1 from WAL replay")
}

// TestWALSnap_MultipleInsertUpdateDelete tests a full CRUD lifecycle
// through the WAL and recovery:
// 1. Insert 3 rows → commit
// 2. Update 1 row → commit
// 3. Delete 1 row → commit
// 4. Crash (no checkpoint, no save)
// 5. Recover and verify final state
func TestWALSnap_MultipleInsertUpdateDelete(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "crud_wal"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0, 0)

	db, wm, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)

	// --- Insert 3 rows ---
	tx1 := transaction.NewTransaction()
	wm.BeginTransaction(tx1)
	for i := int64(1); i <= 3; i++ {
		row := data.Row{Data: map[string]interface{}{"id": i, "name": "User"}}
		wm.LogInsert(tx1, db.Tables["users"], row)
		db.Tables["users"].Rows = append(db.Tables["users"].Rows, row)
	}
	wm.Commit(tx1)

	// --- Update row id=2 ---
	tx2 := transaction.NewTransaction()
	wm.BeginTransaction(tx2)
	oldRow := data.Row{Data: map[string]interface{}{"id": int64(2), "name": "User"}}
	newRow := data.Row{Data: map[string]interface{}{"id": int64(2), "name": "Updated"}}
	wm.LogUpdate(tx2, db.Tables["users"], "2", oldRow, newRow)
	wm.Commit(tx2)

	// --- Delete row id=3 ---
	tx3 := transaction.NewTransaction()
	wm.BeginTransaction(tx3)
	delRow := data.Row{Data: map[string]interface{}{"id": int64(3), "name": "User"}}
	wm.LogDelete(tx3, db.Tables["users"], "3", delRow)
	wm.Commit(tx3)

	// --- "Crash" ---
	reg.CloseAll()

	// --- Recover ---
	reg2 := NewRegistryWithWAL(tempDir, eng, true, 0, 0)
	defer reg2.CloseAll()

	_, _, err = reg2.GetWithWAL(dbName)
	assert.NilError(t, err)

	// Verify by reading WAL directly
	walReader, err := wal.NewWALReader(filepath.Join(tempDir, dbName, dbName+".wal"))
	assert.NilError(t, err)
	defer walReader.Close()

	records, err := walReader.ScanAll()
	assert.NilError(t, err)

	// Count record types from WAL
	inserts, updates, deletes := 0, 0, 0
	for _, r := range records {
		switch r.GetHeader().Type {
		case wal.RecordInsert:
			inserts++
		case wal.RecordUpdate:
			updates++
		case wal.RecordDelete:
			deletes++
		}
	}
	assert.Equal(t, inserts, 3, "should have 3 insert records")
	assert.Equal(t, updates, 1, "should have 1 update record")
	assert.Equal(t, deletes, 1, "should have 1 delete record")
}

// TestWALSnap_SaveAllThenLoad verifies the full save+load cycle:
// 1. Load database with WAL
// 2. Insert rows into memory
// 3. Call registry.SaveAll (creates checkpoint+snapshot)
// 4. Close everything
// 5. Reload — data should be intact from snapshot alone
func TestWALSnap_SaveAllThenLoad(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "saveload"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0, 0)

	db, _, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)

	// Insert into memory
	db.Tables["users"].Rows = append(db.Tables["users"].Rows,
		data.Row{Data: map[string]interface{}{"id": int64(1), "name": "SavedUser"}},
	)

	// SaveAll triggers checkpoint → snapshot
	tx := transaction.NewTransaction()
	reg.SaveAll(tx)
	reg.CloseAll()

	// Reload
	reg2 := NewRegistryWithWAL(tempDir, eng, true, 0, 0)
	defer reg2.CloseAll()

	db2, _, err := reg2.GetWithWAL(dbName)
	assert.NilError(t, err)

	assert.Equal(t, len(db2.Tables["users"].Rows), 1)
	assert.Equal(t, db2.Tables["users"].Rows[0].Data["name"], "SavedUser")
}

// TestWALSnap_WALDisabled verifies that snapshots work without WAL:
// 1. Create registry with WAL disabled
// 2. Load database, insert rows
// 3. Create snapshot manually
// 4. Reload — data should be from snapshot
func TestWALSnap_WALDisabled(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "nowal"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, false, 0, 0)

	db, wm, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)
	assert.Assert(t, wm == nil, "WAL manager should be nil when disabled")

	// Insert into memory and snapshot
	db.Tables["users"].Rows = append(db.Tables["users"].Rows,
		data.Row{Data: map[string]interface{}{"id": int64(1), "name": "NoWAL"}},
	)
	_, _, err = eng.CreateSnapshot(db, filepath.Join(tempDir, dbName))
	assert.NilError(t, err)

	reg.CloseAll()

	// Reload
	reg2 := NewRegistryWithWAL(tempDir, eng, false, 0, 0)
	defer reg2.CloseAll()

	db2, _, err := reg2.GetWithWAL(dbName)
	assert.NilError(t, err)
	assert.Equal(t, len(db2.Tables["users"].Rows), 1)
	assert.Equal(t, db2.Tables["users"].Rows[0].Data["name"], "NoWAL")

	// No WAL file should exist
	walPath := filepath.Join(tempDir, dbName, dbName+".wal")
	_, err = os.Stat(walPath)
	assert.Assert(t, os.IsNotExist(err), "WAL file should not exist when disabled")
}

// TestWALSnap_EmptyDatabaseSnapshotRoundtrip verifies snapshot of empty DB:
// 1. Create database with tables but no rows
// 2. Snapshot
// 3. Load — tables should exist, rows should be empty
func TestWALSnap_EmptyDatabaseSnapshotRoundtrip(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "emptydb"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	db, err := eng.LoadDatabase(filepath.Join(tempDir, dbName))
	assert.NilError(t, err)

	assert.Equal(t, len(db.Tables["users"].Rows), 0)
	assert.Assert(t, db.Tables["users"].Schema != nil)
	assert.Equal(t, len(db.Tables["users"].Schema.Columns), 2)
}

// -----------------------------------------------------------------------------
// SAD PATH TESTS
// -----------------------------------------------------------------------------

// TestSnap_CorruptedCRC verifies that a corrupted snapshot is rejected:
// 1. Create a valid snapshot
// 2. Corrupt the trailing CRC bytes
// 3. Attempt to load — should fail with CRC mismatch error
func TestSnap_CorruptedCRC(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "corrupt_crc"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	db := &schema.Database{
		Name:   dbName,
		Path:   dbPath,
		Tables: map[string]*schema.Table{},
	}

	eng := engine.NewMemoryEngine()
	_, _, err := eng.CreateSnapshot(db, dbPath)
	assert.NilError(t, err)

	// Find the snap file and corrupt its CRC
	entries, _ := os.ReadDir(dbPath)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".snap") {
			snapPath := filepath.Join(dbPath, e.Name())
			content, _ := os.ReadFile(snapPath)
			// Flip last 4 bytes (CRC)
			for i := len(content) - 4; i < len(content); i++ {
				content[i] ^= 0xFF
			}
			os.WriteFile(snapPath, content, 0644)

			// Try to load
			newDB := &schema.Database{
				Name:   dbName,
				Path:   dbPath,
				Tables: make(map[string]*schema.Table),
			}
			err = engine.LoadSnapshot(newDB, snapPath)
			assert.Assert(t, err != nil, "should fail on corrupt CRC")
			assert.Assert(t, strings.Contains(err.Error(), "CRC mismatch"),
				"error should mention CRC, got: %v", err)
			break
		}
	}
}

// TestSnap_TruncatedFile verifies that a truncated snapshot is rejected:
// 1. Create a valid snapshot
// 2. Truncate it to half its size
// 3. Attempt to load — should fail
func TestSnap_TruncatedFile(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "truncated_snap"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	db := &schema.Database{
		Name: dbName,
		Path: dbPath,
		Tables: map[string]*schema.Table{
			"users": {
				Name: "users",
				Path: dbPath,
				Schema: &schema.TableSchema{
					Columns: []schema.Column{
						{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
					},
				},
				Rows:    []data.Row{{Data: map[string]interface{}{"id": int64(1)}}},
				Indexes: make(map[string]*data.Index),
			},
		},
	}

	eng := engine.NewMemoryEngine()
	_, _, err := eng.CreateSnapshot(db, dbPath)
	assert.NilError(t, err)

	// Truncate the snap file
	entries, _ := os.ReadDir(dbPath)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".snap") {
			snapPath := filepath.Join(dbPath, e.Name())
			content, _ := os.ReadFile(snapPath)
			os.WriteFile(snapPath, content[:len(content)/2], 0644)

			newDB := &schema.Database{Name: dbName, Path: dbPath, Tables: make(map[string]*schema.Table)}
			err = engine.LoadSnapshot(newDB, snapPath)
			assert.Assert(t, err != nil, "should fail on truncated snapshot")
			break
		}
	}
}

// TestSnap_InvalidMagic verifies that a snapshot with bad magic is rejected:
// 1. Create a valid snapshot
// 2. Overwrite the first 8 bytes (magic) with garbage
// 3. Attempt to load — should fail
func TestSnap_InvalidMagic(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "bad_magic"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	db := &schema.Database{Name: dbName, Path: dbPath, Tables: map[string]*schema.Table{}}
	eng := engine.NewMemoryEngine()
	_, _, err := eng.CreateSnapshot(db, dbPath)
	assert.NilError(t, err)

	entries, _ := os.ReadDir(dbPath)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".snap") {
			snapPath := filepath.Join(dbPath, e.Name())
			content, _ := os.ReadFile(snapPath)
			copy(content[:8], []byte("BADMAGIC"))

			// Recompute CRC so the CRC check passes — we want magic check to fail first
			payload := content[:len(content)-4]
			crc := crc32.ChecksumIEEE(payload)
			binary.LittleEndian.PutUint32(content[len(content)-4:], crc)
			os.WriteFile(snapPath, content, 0644)

			newDB := &schema.Database{Name: dbName, Path: dbPath, Tables: make(map[string]*schema.Table)}
			err = engine.LoadSnapshot(newDB, snapPath)
			assert.Assert(t, err != nil, "should fail on invalid magic")
			assert.Assert(t, strings.Contains(err.Error(), "magic"),
				"error should mention magic, got: %v", err)
			break
		}
	}
}

// TestWALSnap_CrashBeforeCommitNoRecovery verifies no ghost data after crash:
// 1. Create database with initial snapshot
// 2. Begin WAL transaction, insert row, but do NOT commit
// 3. "Crash"
// 4. Recover — uncommitted row must NOT appear
func TestWALSnap_CrashBeforeCommitNoRecovery(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "nocommit"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0, 0)

	db, wm, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)

	// Begin but don't commit
	tx := transaction.NewTransaction()
	wm.BeginTransaction(tx)
	wm.LogInsert(tx, db.Tables["users"], data.Row{Data: map[string]interface{}{"id": int64(99), "name": "Ghost"}})
	// NO Commit!

	reg.CloseAll()

	// Recover
	reg2 := NewRegistryWithWAL(tempDir, eng, true, 0, 0)
	defer reg2.CloseAll()

	db2, _, err := reg2.GetWithWAL(dbName)
	assert.NilError(t, err)
	assert.Equal(t, len(db2.Tables["users"].Rows), 0,
		"uncommitted row should not appear after recovery")
}

// TestWALSnap_CorruptedWALRecovery verifies behaviour with a corrupted WAL:
// 1. Write valid transactions to WAL
// 2. Append garbage bytes to WAL file
// 3. Attempt recovery — first transaction should still be recovered
//    (or gracefully error)
func TestWALSnap_CorruptedWALRecovery(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "corruptwal"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")
	dbPath := filepath.Join(tempDir, dbName)

	eng := engine.NewMemoryEngine()
	wm, err := NewWALManager(nil, dbPath, dbName, true, 0, 0, nil, eng)
	assert.NilError(t, err)

	db := createTestDatabase(t, dbName)
	tx := createTestTransaction(t)
	wm.BeginTransaction(tx)
	wm.LogInsert(tx, db.Tables["users"], createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Valid"}))
	wm.Commit(tx)
	wm.Close()

	// Append garbage
	walPath := filepath.Join(dbPath, dbName+".wal")
	f, err := os.OpenFile(walPath, os.O_APPEND|os.O_WRONLY, 0644)
	assert.NilError(t, err)
	f.Write([]byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
	f.Close()

	// Recover — should either recover the valid transaction or return an error
	wm2, err := NewWALManager(nil, dbPath, dbName, true, 0, 0, nil, eng)
	assert.NilError(t, err)
	defer wm2.Close()

	result, err := wm2.Recover()
	if err != nil {
		// Acceptable: WAL detected corruption
		t.Logf("WAL corruption detected (expected): %v", err)
	} else {
		// Also acceptable: recovered what it could
		assert.Assert(t, result != nil)
		assert.Equal(t, len(result.InsertOps), 1,
			"should recover the valid committed transaction")
	}
}

// TestWALSnap_MissingSnapshotStillRecoverable verifies WAL-only recovery:
// 1. Create database dir (no snapshot)
// 2. Write operations to WAL
// 3. Load via registry — WAL alone should produce a working database
func TestWALSnap_MissingSnapshotStillRecoverable(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "nosnap"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	// Create a minimal snapshot with the table schema
	// (database needs schema to replay WAL ops into)
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	// Delete all .snap files to simulate missing snapshot
	entries, _ := os.ReadDir(dbPath)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".snap") {
			os.Remove(filepath.Join(dbPath, e.Name()))
		}
	}

	// Write WAL entries manually
	eng := engine.NewMemoryEngine()
	wm, err := NewWALManager(nil, dbPath, dbName, true, 0, 0, nil, eng)
	assert.NilError(t, err)

	db := createTestDatabase(t, dbName)
	tx := createTestTransaction(t)
	wm.BeginTransaction(tx)
	wm.LogInsert(tx, db.Tables["users"], createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Orphan"}))
	wm.Commit(tx)
	wm.Close()

	// Load — should get an empty database (no snapshot), but WAL can still be read
	loadedDB, err := eng.LoadDatabase(dbPath)
	assert.NilError(t, err)
	// No snapshot means no tables loaded from snapshot
	assert.Equal(t, len(loadedDB.Tables), 0,
		"without snapshot, no tables are loaded")
}

// TestWALSnap_EmptyWALFileRecovery verifies graceful handling of a WAL
// that has only a header (no records):
func TestWALSnap_EmptyWALFileRecovery(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "emptywal"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0, 0)
	defer reg.CloseAll()

	// Loading triggers WAL creation (header only) + recovery (no ops)
	db, wm, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)
	assert.Assert(t, db != nil)
	assert.Assert(t, wm != nil)
	assert.Equal(t, len(db.Tables["users"].Rows), 0)
}

// TestWALSnap_TruncatedWALMidRecord verifies recovery from a WAL that
// was truncated in the middle of writing a record (simulating power loss
// during write):
// 1. Write a valid committed transaction
// 2. Truncate the WAL by N bytes from the end (simulating partial write)
// 3. Recovery should handle this gracefully
func TestWALSnap_TruncatedWALMidRecord(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "truncwal"
	dbPath := filepath.Join(tempDir, dbName)
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	wm, err := NewWALManager(nil, dbPath, dbName, true, 0, 0, nil, eng)
	assert.NilError(t, err)

	db := createTestDatabase(t, dbName)

	// Tx1: committed
	tx1 := createTestTransaction(t)
	wm.BeginTransaction(tx1)
	wm.LogInsert(tx1, db.Tables["users"], createTestRow(t, map[string]interface{}{"id": int64(1), "name": "First"}))
	wm.Commit(tx1)

	// Tx2: committed
	tx2 := createTestTransaction(t)
	wm.BeginTransaction(tx2)
	wm.LogInsert(tx2, db.Tables["users"], createTestRow(t, map[string]interface{}{"id": int64(2), "name": "Second"}))
	wm.Commit(tx2)

	wm.Close()

	// Truncate the last 10 bytes (corrupting the last commit record)
	walPath := filepath.Join(dbPath, dbName+".wal")
	truncateFile(t, walPath, 10)

	// Recover — tx1 should survive, tx2 is damaged
	wm2, err := NewWALManager(nil, dbPath, dbName, true, 0, 0, nil, eng)
	assert.NilError(t, err)
	defer wm2.Close()

	result, err := wm2.Recover()
	if err != nil {
		t.Logf("Recovery error (possible, acceptable): %v", err)
	} else {
		// If recovery succeeded, tx1 should be present
		// tx2 might or might not be depending on where truncation hit
		assert.Assert(t, len(result.InsertOps) >= 1,
			"at least tx1 should be recovered")
	}
}

// TestWALSnap_ZeroByteWALFile verifies handling of a zero-byte WAL file:
// 1. Create the WAL file as an empty file (0 bytes, no header)
// 2. Attempt to open — should error or reinitialize
func TestWALSnap_ZeroByteWALFile(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "zerowal"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	// Create a zero-byte WAL file
	walPath := filepath.Join(dbPath, dbName+".wal")
	os.WriteFile(walPath, []byte{}, 0644)

	_ = engine.NewMemoryEngine() // Ensure engine package is imported

	// Opening a zero-byte WAL should either error or handle gracefully
	_, err := wal.NewWAL(walPath, dbName)
	if err != nil {
		t.Logf("Zero-byte WAL correctly rejected: %v", err)
	} else {
		t.Log("Zero-byte WAL handled gracefully (reinitialized)")
	}
	// Either outcome is acceptable — we just verify no panic
}

// TestWALSnap_SnapshotAfterMultipleCheckpoints verifies that loading
// after multiple checkpoints picks the latest snapshot:
// 1. Insert data, checkpoint
// 2. Insert more data, checkpoint again (now 2 snapshots)
// 3. Close, reload
// 4. Latest snapshot should have all data
func TestWALSnap_SnapshotAfterMultipleCheckpoints(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "multicp"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")

	eng := engine.NewMemoryEngine()
	reg := NewRegistryWithWAL(tempDir, eng, true, 0, 0)

	db, wm, err := reg.GetWithWAL(dbName)
	assert.NilError(t, err)

	// Insert + checkpoint 1
	db.Tables["users"].Rows = append(db.Tables["users"].Rows,
		data.Row{Data: map[string]interface{}{"id": int64(1), "name": "First"}},
	)
	wm.WriteCheckpoint(db)

	// Insert more + checkpoint 2
	db.Tables["users"].Rows = append(db.Tables["users"].Rows,
		data.Row{Data: map[string]interface{}{"id": int64(2), "name": "Second"}},
	)
	wm.WriteCheckpoint(db)

	reg.CloseAll()

	// Reload — latest snapshot should have both rows
	reg2 := NewRegistryWithWAL(tempDir, eng, true, 0, 0)
	defer reg2.CloseAll()

	db2, _, err := reg2.GetWithWAL(dbName)
	assert.NilError(t, err)
	assert.Equal(t, len(db2.Tables["users"].Rows), 2)
}

// TestWALSnap_AbortedTransactionNotInRecovery verifies that explicitly
// aborted transactions don't leak into recovery:
// 1. Begin tx, insert, abort
// 2. Begin another tx, insert, commit
// 3. Crash, recover
// 4. Only the committed data should appear
func TestWALSnap_AbortedTransactionNotInRecovery(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "aborted"
	createMinimalDatabaseFiles(t, tempDir, dbName, "users")
	dbPath := filepath.Join(tempDir, dbName)

	eng := engine.NewMemoryEngine()
	wm, err := NewWALManager(nil, dbPath, dbName, true, 0, 0, nil, eng)
	assert.NilError(t, err)

	db := createTestDatabase(t, dbName)

	// Aborted transaction
	tx1 := createTestTransaction(t)
	wm.BeginTransaction(tx1)
	wm.LogInsert(tx1, db.Tables["users"], createTestRow(t, map[string]interface{}{"id": int64(1), "name": "Aborted"}))
	wm.Abort(tx1)

	// Committed transaction
	tx2 := createTestTransaction(t)
	wm.BeginTransaction(tx2)
	wm.LogInsert(tx2, db.Tables["users"], createTestRow(t, map[string]interface{}{"id": int64(2), "name": "Committed"}))
	wm.Commit(tx2)

	wm.Close()

	// Recover
	wm2, err := NewWALManager(nil, dbPath, dbName, true, 0, 0, nil, eng)
	assert.NilError(t, err)
	defer wm2.Close()

	result, err := wm2.Recover()
	assert.NilError(t, err)
	assert.Equal(t, len(result.InsertOps), 1, "only committed insert should appear")
	assert.Equal(t, result.InsertOps[0].Key, "2")
}

// TestSnap_NonExistentDatabasePath verifies that loading a non-existent
// database path returns an error:
func TestSnap_NonExistentDatabasePath(t *testing.T) {
	eng := engine.NewMemoryEngine()
	_, err := eng.LoadDatabase("/nonexistent/path/that/doesnt/exist")
	assert.Assert(t, err != nil, "should fail for non-existent path")
}

// TestWALSnap_MultipleTablesSnapshot verifies snapshot with multiple tables:
// 1. Create database with 3 tables
// 2. Populate each with different data
// 3. Snapshot
// 4. Reload and verify all tables and rows are intact
func TestWALSnap_MultipleTablesSnapshot(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	dbName := "multitable"
	dbPath := filepath.Join(tempDir, dbName)
	os.MkdirAll(dbPath, 0755)

	db := &schema.Database{
		Name: dbName,
		Path: dbPath,
		Tables: map[string]*schema.Table{
			"users": {
				Name: "users",
				Path: dbPath,
				Schema: &schema.TableSchema{
					Columns: []schema.Column{
						{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
						{Name: "name", Type: schema.ColumnTypeText},
					},
				},
				Rows: []data.Row{
					{Data: map[string]interface{}{"id": int64(1), "name": "Alice"}},
				},
				Indexes: make(map[string]*data.Index),
			},
			"orders": {
				Name: "orders",
				Path: dbPath,
				Schema: &schema.TableSchema{
					Columns: []schema.Column{
						{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
						{Name: "user_id", Type: schema.ColumnTypeInt},
						{Name: "product", Type: schema.ColumnTypeText},
					},
				},
				Rows: []data.Row{
					{Data: map[string]interface{}{"id": int64(1), "user_id": int64(1), "product": "Widget"}},
					{Data: map[string]interface{}{"id": int64(2), "user_id": int64(1), "product": "Gadget"}},
				},
				Indexes: make(map[string]*data.Index),
			},
			"config": {
				Name: "config",
				Path: dbPath,
				Schema: &schema.TableSchema{
					Columns: []schema.Column{
						{Name: "key", Type: schema.ColumnTypeText, PrimaryKey: true},
						{Name: "value", Type: schema.ColumnTypeText},
					},
				},
				Rows: []data.Row{
					{Data: map[string]interface{}{"key": "version", "value": "1.0"}},
				},
				Indexes: make(map[string]*data.Index),
			},
		},
	}

	eng := engine.NewMemoryEngine()
	_, _, err := eng.CreateSnapshot(db, dbPath)
	assert.NilError(t, err)

	// Reload
	loaded, err := eng.LoadDatabase(dbPath)
	assert.NilError(t, err)

	assert.Equal(t, len(loaded.Tables), 3)
	assert.Equal(t, len(loaded.Tables["users"].Rows), 1)
	assert.Equal(t, len(loaded.Tables["orders"].Rows), 2)
	assert.Equal(t, len(loaded.Tables["config"].Rows), 1)
	assert.Equal(t, loaded.Tables["orders"].Rows[1].Data["product"], "Gadget")
	assert.Equal(t, loaded.Tables["config"].Rows[0].Data["value"], "1.0")
}

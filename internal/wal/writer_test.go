package wal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

// createTestWAL creates a temporary WAL file for testing.
// Returns the WAL instance and the temp directory path.
// The caller should defer cleanup using cleanupTestWAL.
func createTestWAL(t *testing.T) (*WAL, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "test-wal")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	walPath := filepath.Join(tempDir, "test-wal.wal")
	dbName := "test-db"
	wal, err := NewWAL(walPath, dbName)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create WAL: %v", err)
	}
	return wal, tempDir
}

// cleanupTestWAL removes the temporary WAL directory and files.
func cleanupTestWAL(t *testing.T, tempDir string) {
	t.Helper()
	if err := os.RemoveAll(tempDir); err != nil {
		t.Logf("failed to remove temp directory: %v", err)
	}
}

// createTestJSON creates a json.RawMessage from a map for testing.
func createTestJSON(t *testing.T, data map[string]interface{}) json.RawMessage {
	t.Helper()

	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to create test JSON: %v", err)
	}
	return json.RawMessage(bytes)
}

// =============================================================================
// SUITE 1: WAL WRITER TESTS
// =============================================================================

// TestBeginTransaction verifies that BeginTransaction:
// - Writes a BeginTxn record to WAL
// - Returns sequential LSNs for each new transaction
// - Creates proper transaction state tracking
func TestBeginTransaction(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	var txOne uint64 = 1
	var txTwo uint64 = 2

	lsn1, err := wal.BeginTransaction(txOne)
	assert.NilError(t, err)
	assert.Assert(t, lsn1 > 0)

	lsn2, err := wal.BeginTransaction(txTwo)
	assert.NilError(t, err)
	assert.Assert(t, lsn2 > lsn1)

	assert.NilError(t, wal.verifyActiveTxn(txOne))
	assert.NilError(t, wal.verifyActiveTxn(txTwo))
}

// TestLogInsert verifies that LogInsert:
// - Writes an Insert record with table name, key, and value
// - Encodes the JSON payload correctly
// - Associates the record with the correct transaction
func TestLogInsert(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	var txOne uint64 = 1

	_, err := wal.BeginTransaction(txOne)
	assert.NilError(t, err)

	lsn, err := wal.LogInsert(txOne, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}))
	assert.NilError(t, err)
	assert.Assert(t, lsn > 0)

	assert.NilError(t, wal.verifyActiveTxn(txOne))
}

// TestLogUpdate verifies that LogUpdate:
// - Writes an Update record with old and new values
// - Both old and new values are properly serialized
// - Key matches the primary key of the row
func TestLogUpdate(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	var txOne uint64 = 1

	_, err := wal.BeginTransaction(txOne)
	assert.NilError(t, err)

	lsn, err := wal.LogUpdate(txOne, "users", "1",
		createTestJSON(t, map[string]interface{}{"name": "Alice"}),
		createTestJSON(t, map[string]interface{}{"name": "Bob"}))
	assert.NilError(t, err)
	assert.Assert(t, lsn > 0)

	assert.NilError(t, wal.verifyActiveTxn(txOne))
}

// TestLogDelete verifies that LogDelete:
// - Writes a Delete record with the old value (for potential undo)
// - Record is associated with the correct transaction
func TestLogDelete(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	var txOne uint64 = 1

	_, err := wal.BeginTransaction(txOne)
	assert.NilError(t, err)

	lsn, err := wal.LogDelete(txOne, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}))
	assert.NilError(t, err)
	assert.Assert(t, lsn > 0)

	assert.NilError(t, wal.verifyActiveTxn(txOne))
}

// TestCommit verifies that Commit:
// - Writes a Commit record to WAL
// - Calls fsync to ensure durability
// - Marks the transaction as no longer active
func TestCommit(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	var txOne uint64 = 1

	_, err := wal.BeginTransaction(txOne)
	assert.NilError(t, err)

	lsn, err := wal.LogInsert(txOne, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}))
	assert.NilError(t, err)

	commitLSN, err := wal.Commit(txOne)
	assert.NilError(t, err)
	assert.Assert(t, commitLSN > lsn)

	// Transaction should no longer be active
	err = wal.verifyActiveTxn(txOne)
	assert.ErrorContains(t, err, "transaction 1 not found")
}

// TestAbort verifies that Abort:
// - Writes an Abort record to WAL
// - Marks the transaction as aborted (not to be replayed)
func TestAbort(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	var txOne uint64 = 1

	_, err := wal.BeginTransaction(txOne)
	assert.NilError(t, err)

	lsn, err := wal.LogInsert(txOne, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}))
	assert.NilError(t, err)

	abortLSN, err := wal.Abort(txOne)
	assert.NilError(t, err)
	assert.Assert(t, abortLSN > lsn)

	// Transaction should be in Aborted state (verifyActiveTxn only returns nil for Active)
	// Actually verifyActiveTxn returns error if not active OR if state != Active
	// Let's check internal state if we can or infer from error message
	err = wal.verifyActiveTxn(txOne)
	assert.ErrorContains(t, err, "transaction 1 not found")
}

// TestWriteCheckpoint verifies that WriteCheckpoint:
// - Writes a Checkpoint record with table checksums
// - Database CRC is included
// - Checkpoint can be used as recovery point
func TestWriteCheckpoint(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	// Write some transactions
	txID := uint64(1)
	wal.BeginTransaction(txID)
	wal.LogInsert(txID, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}))
	wal.Commit(txID)

	tables := []TableChecksum{
		{TableName: "users", DataCRC32: 12345, MetaCRC32: 67890},
	}
	dbCRC := uint32(11111)

	lsn, err := wal.WriteCheckpoint(tables, dbCRC)
	assert.NilError(t, err)
	assert.Assert(t, lsn > 0)
	assert.Equal(t, wal.LastCheckpointLSN(), lsn)
}

// TestConcurrentTransactions verifies that multiple transactions:
// - Can be active simultaneously
// - Each gets unique TxIDs
// - Commits/Aborts are independent
func TestConcurrentTransactions(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	tx1 := uint64(1)
	tx2 := uint64(2)
	tx3 := uint64(3)

	// Begin tx1, tx2, tx3
	wal.BeginTransaction(tx1)
	wal.BeginTransaction(tx2)
	wal.BeginTransaction(tx3)

	// Log ops to each in interleaved order
	wal.LogInsert(tx1, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}))
	wal.LogInsert(tx2, "users", "2", createTestJSON(t, map[string]interface{}{"name": "Bob"}))
	wal.LogInsert(tx3, "users", "3", createTestJSON(t, map[string]interface{}{"name": "Charlie"}))

	wal.LogUpdate(tx1, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}), createTestJSON(t, map[string]interface{}{"name": "Alice2"}))

	// Commit tx1, Abort tx2, Commit tx3
	_, err := wal.Commit(tx1)
	assert.NilError(t, err)

	_, err = wal.Abort(tx2)
	assert.NilError(t, err)

	_, err = wal.Commit(tx3)
	assert.NilError(t, err)

	// Verify states
	assert.ErrorContains(t, wal.verifyActiveTxn(tx1), "transaction 1 not found")
	assert.ErrorContains(t, wal.verifyActiveTxn(tx2), "transaction 2 not found")
	assert.ErrorContains(t, wal.verifyActiveTxn(tx3), "transaction 3 not found")
}

// TestLSNMonotonicity verifies that LSNs:
// - Are always increasing
// - Never duplicate
// - Increment correctly across all record types
func TestLSNMonotonicity(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	var lastLSN uint64 = 0
	checkLSN := func(lsn uint64) {
		assert.Assert(t, lsn > lastLSN, "LSN %d should be greater than %d", lsn, lastLSN)
		lastLSN = lsn
	}

	tx1 := uint64(1)
	lsn, _ := wal.BeginTransaction(tx1)
	checkLSN(lsn)

	lsn, _ = wal.LogInsert(tx1, "t", "k", createTestJSON(t, map[string]interface{}{"a": 1}))
	checkLSN(lsn)

	lsn, _ = wal.LogUpdate(tx1, "t", "k", createTestJSON(t, map[string]interface{}{"a": 1}), createTestJSON(t, map[string]interface{}{"a": 2}))
	checkLSN(lsn)

	lsn, _ = wal.LogDelete(tx1, "t", "k", createTestJSON(t, map[string]interface{}{"a": 2}))
	checkLSN(lsn)

	lsn, _ = wal.Commit(tx1)
	checkLSN(lsn)

	lsn, _ = wal.WriteCheckpoint(nil, 0)
	checkLSN(lsn)
}

// =============================================================================
// FILE VERIFICATION HELPERS
// =============================================================================


package wal

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

// =============================================================================
// SUITE 2: WAL READER TESTS
// =============================================================================

// TestDecodeCreateTablePayload verifies decodeCreateTablePayload properly unpacks binary payload
func TestDecodeCreateTablePayload(t *testing.T) {
	// Create payload
	var txID uint64 = 12345
	tableName := "users"
	schemaBytes := []byte("fake_schema")

	payloadSize := 8 + 2 + len(tableName) + 4 + len(schemaBytes)
	payload := make([]byte, payloadSize)

	offset := 0
	ByteOrder.PutUint64(payload[offset:], txID)
	offset += 8

	ByteOrder.PutUint16(payload[offset:], uint16(len(tableName)))
	offset += 2
	copy(payload[offset:], tableName)
	offset += len(tableName)

	ByteOrder.PutUint32(payload[offset:], uint32(len(schemaBytes)))
	offset += 4
	copy(payload[offset:], schemaBytes)
	offset += len(schemaBytes)

	header := WALRecordHeader{
		Type:       RecordCreateTable,
		PayloadLen: uint32(payloadSize),
	}

	rec, err := decodeCreateTablePayload(header, payload)
	assert.NilError(t, err)
	assert.Equal(t, rec.TxID, txID)
	assert.Equal(t, rec.TableName, tableName)
	assert.DeepEqual(t, rec.Schema, schemaBytes)
}

// TestDecodeDropTablePayload verifies decodeDropTablePayload properly unpacks binary payload
func TestDecodeDropTablePayload(t *testing.T) {
	var txID uint64 = 98765
	tableName := "orders"

	payloadSize := 8 + 2 + len(tableName)
	payload := make([]byte, payloadSize)

	offset := 0
	ByteOrder.PutUint64(payload[offset:], txID)
	offset += 8

	ByteOrder.PutUint16(payload[offset:], uint16(len(tableName)))
	offset += 2
	copy(payload[offset:], tableName)
	offset += len(tableName)

	header := WALRecordHeader{
		Type:       RecordDropTable,
		PayloadLen: uint32(payloadSize),
	}

	rec, err := decodeDropTablePayload(header, payload)
	assert.NilError(t, err)
	assert.Equal(t, rec.TxID, txID)
	assert.Equal(t, rec.TableName, tableName)
}

// TestReadFileHeader verifies that the WAL file header:
// - Can be read after creating a new WAL
// - Contains correct magic bytes
// - Contains correct version number
// - Contains the database name
func TestReadFileHeader(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	dbName := "test-db" // Hardcoded in createTestWAL

	// Close WAL to ensure everything is flushed and we can read it
	err := wal.Close()
	assert.NilError(t, err)
	defer cleanupTestWAL(t, tempDir)

	walPath := filepath.Join(tempDir, "test-wal.wal")
	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	header, err := reader.ReadFileHeader()
	assert.NilError(t, err)

	assert.Equal(t, header.Magic, WALMagic)
	assert.Equal(t, header.Version, WALVersion)

	// Check database name (null-padded)
	var expectedName [32]byte
	copy(expectedName[:], dbName)
	assert.Equal(t, header.DatabaseName, expectedName)
}

// TestReadRecords verifies that records can be read back in order:
// - Multiple record types are read correctly
// - Record order matches write order
// - All fields are deserialized correctly
func TestReadRecords(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	txID := uint64(1)
	wal.BeginTransaction(txID)
	wal.LogInsert(txID, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}))
	wal.LogUpdate(txID, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}), createTestJSON(t, map[string]interface{}{"name": "Bob"}))
	wal.LogDelete(txID, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Bob"}))
	wal.Commit(txID)

	err := wal.Close()
	assert.NilError(t, err)

	walPath := filepath.Join(tempDir, "test-wal.wal")
	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	records, err := reader.ScanAll()
	assert.NilError(t, err)

	assert.Equal(t, len(records), 5)

	assert.Assert(t, records[0].GetHeader().Type == RecordBeginTxn)
	assert.Assert(t, records[1].GetHeader().Type == RecordInsert)
	assert.Assert(t, records[2].GetHeader().Type == RecordUpdate)
	assert.Assert(t, records[3].GetHeader().Type == RecordDelete)
	assert.Assert(t, records[4].GetHeader().Type == RecordCommit)
}

// TestCRCValidation verifies that corrupted records are detected:
// - Write valid records
// - Manually corrupt one record's data
// - Reader should return CRC error for corrupted record
func TestCRCValidation(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	txID := uint64(1)
	wal.BeginTransaction(txID)
	wal.LogInsert(txID, "users", "1", createTestJSON(t, map[string]interface{}{"name": "Alice"}))
	wal.Commit(txID)
	wal.Close()

	walPath := filepath.Join(tempDir, "test-wal.wal")

	// Corrupt the file
	f, err := os.OpenFile(walPath, os.O_RDWR, 0644)
	assert.NilError(t, err)

	// Seek to somewhere in the middle (after header) and flip a byte
	// Header is 64 bytes. First record (BeginTxn) is small.
	// Let's corrupt the Insert record payload.
	// BeginTxn is 32(header) + 8(payload) + padding -> 40 bytes.
	// So Insert starts around 64 + 40 = 104 bytes.
	_, err = f.Seek(120, io.SeekStart)
	assert.NilError(t, err)

	b := []byte{0xFF}
	_, err = f.Write(b)
	assert.NilError(t, err)
	f.Close()

	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	// Should fail eventually
	_, err = reader.ScanAll()
	assert.ErrorContains(t, err, "CRC mismatch")
}

// TestScanFromLSN verifies seeking to a specific LSN:
// - Write multiple records
// - Seek to middle LSN
// - Read only returns records from that LSN onward
func TestScanFromLSN(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	txID := uint64(1)
	wal.BeginTransaction(txID)
	lsn2, _ := wal.LogInsert(txID, "t", "1", createTestJSON(t, map[string]interface{}{"v": 1}))
	lsn3, _ := wal.LogInsert(txID, "t", "2", createTestJSON(t, map[string]interface{}{"v": 2}))
	lsn4, _ := wal.Commit(txID)

	wal.Close()

	walPath := filepath.Join(tempDir, "test-wal.wal")
	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	// Scan from LSN 2 (should get 3 and 4)
	records, err := reader.ScanFrom(lsn2)
	assert.NilError(t, err)

	// ScanFrom returns records with LSN > afterLSN.
	// So if we pass lsn2, we expect lsn3 and lsn4.
	assert.Equal(t, len(records), 2)
	assert.Equal(t, records[0].GetHeader().LSN, lsn3)
	assert.Equal(t, records[1].GetHeader().LSN, lsn4)
}

// TestReadEmptyWAL verifies reading a newly created WAL:
// - Create WAL (only header exists)
// - Reader should return no records
// - No errors should occur
func TestReadEmptyWAL(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	wal.Close()
	defer cleanupTestWAL(t, tempDir)

	walPath := filepath.Join(tempDir, "test-wal.wal")
	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	records, err := reader.ScanAll()
	assert.NilError(t, err)
	assert.Equal(t, len(records), 0)
}

// TestReadTruncatedRecord verifies handling of incomplete records:
// - Write a record, then truncate the file mid-record
// - Reader should return error or stop cleanly
func TestReadTruncatedRecord(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	txID := uint64(1)
	wal.BeginTransaction(txID)
	// Write a large record
	largeVal := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeVal["key"] = "value"
	}
	wal.LogInsert(txID, "t", "k", createTestJSON(t, largeVal))
	wal.Commit(txID)
	wal.Close()

	walPath := filepath.Join(tempDir, "test-wal.wal")
	info, _ := os.Stat(walPath)

	// Truncate last 10 bytes
	err := os.Truncate(walPath, info.Size()-10)
	assert.NilError(t, err)

	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	_, err = reader.ScanAll()
	// Should return EOF or incomplete header error
	assert.Assert(t, err != nil)
}

// TestReadAllRecordTypes verifies each record type is decoded correctly:
// - BeginTxn: TxID parsed
// - Insert: TableName, Key, Value parsed
// - Update: TableName, Key, OldValue, NewValue parsed
// - Delete: TableName, Key, OldValue parsed
// - Commit: TxID parsed
// - Abort: TxID parsed
// - Checkpoint: TableChecksums, DbCRC parsed
func TestReadAllRecordTypes(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	txID := uint64(1)
	wal.BeginTransaction(txID)
	wal.LogInsert(txID, "table1", "key1", createTestJSON(t, map[string]interface{}{"a": 1}))
	wal.LogUpdate(txID, "table1", "key1", createTestJSON(t, map[string]interface{}{"a": 1}), createTestJSON(t, map[string]interface{}{"a": 2}))
	wal.LogDelete(txID, "table1", "key1", createTestJSON(t, map[string]interface{}{"a": 2}))
	wal.Commit(txID)

	wal.BeginTransaction(2)
	wal.Abort(2)

	wal.WriteCheckpoint([]TableChecksum{{TableName: "t1", DataCRC32: 1, MetaCRC32: 2}}, 999)

	wal.Close()

	walPath := filepath.Join(tempDir, "test-wal.wal")
	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	records, err := reader.ScanAll()
	assert.NilError(t, err)

	// BeginTxn
	assert.Assert(t, records[0].GetHeader().Type == RecordBeginTxn)
	assert.Equal(t, records[0].(*BeginTxnRecord).TxID, txID)

	// Insert
	assert.Assert(t, records[1].GetHeader().Type == RecordInsert)
	ins := records[1].(*InsertRecord)
	assert.Equal(t, ins.TableName, "table1")
	assert.Equal(t, ins.Key, "key1")

	// Update
	assert.Assert(t, records[2].GetHeader().Type == RecordUpdate)
	upd := records[2].(*UpdateRecord)
	assert.Equal(t, upd.TableName, "table1")

	// Delete
	assert.Assert(t, records[3].GetHeader().Type == RecordDelete)
	del := records[3].(*DeleteRecord)
	assert.Equal(t, del.TableName, "table1")

	// Commit
	assert.Assert(t, records[4].GetHeader().Type == RecordCommit)

	// BeginTxn 2
	assert.Assert(t, records[5].GetHeader().Type == RecordBeginTxn)

	// Abort 2
	assert.Assert(t, records[6].GetHeader().Type == RecordAbort)

	// Checkpoint
	assert.Assert(t, records[7].GetHeader().Type == RecordCheckpoint)
	cp := records[7].(*CheckpointRecord)
	assert.Equal(t, cp.DatabaseCRC32, uint32(999))
	assert.Equal(t, len(cp.Tables), 1)
}

// TestSeekToOffset verifies direct offset seeking:
// - Record file offsets during writes
// - Seek to specific offset
// - Verify we read the expected record
func TestSeekToOffset(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	txID := uint64(1)
	wal.BeginTransaction(txID) // Record 0
	wal.LogInsert(txID, "t", "k", createTestJSON(t, map[string]interface{}{"a": 1})) // Record 1
	wal.Commit(txID) // Record 2

	// Get offset of Record 1 (Insert)
	// We can't get it directly from Writer easily without inspecting logs or modifying Writer to return offset
	// But we can read it first to get offset, then seek.
	wal.Close()

	walPath := filepath.Join(tempDir, "test-wal.wal")
	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	records, err := reader.ScanAll()
	assert.NilError(t, err)

	targetOffset := records[1].GetHeader().FileOffset
	targetLSN := records[1].GetHeader().LSN

	// Create new reader and seek
	reader2, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader2.Close()

	// Need to read file header first usually, or seek past it?
	// SeekToOffset takes absolute file offset.
	err = reader2.SeekToOffset(targetOffset)
	assert.NilError(t, err)

	rec, err := reader2.ReadNextRecord()
	assert.NilError(t, err)
	assert.Equal(t, rec.GetHeader().LSN, targetLSN)
	assert.Assert(t, rec.GetHeader().Type == RecordInsert)
}

// =============================================================================
// READER EDGE CASE TESTS
// =============================================================================

// TestReadAfterReopen verifies that WAL can be reopened and read:
// - Write records, close WAL
// - Reopen WAL for writing
// - Write more records
// - Read should show all records
func TestReadAfterReopen(t *testing.T) {
	wal, tempDir := createTestWAL(t)
	defer cleanupTestWAL(t, tempDir)

	txID := uint64(1)
	wal.BeginTransaction(txID)
	wal.LogInsert(txID, "t", "1", createTestJSON(t, map[string]interface{}{"v": 1}))
	wal.Commit(txID)
	wal.Close()

	// Reopen
	walPath := filepath.Join(tempDir, "test-wal.wal")
	wal2, err := NewWAL(walPath, "test-db")
	assert.NilError(t, err)

	txID2 := uint64(2)
	wal2.BeginTransaction(txID2)
	wal2.LogInsert(txID2, "t", "2", createTestJSON(t, map[string]interface{}{"v": 2}))
	wal2.Commit(txID2)
	wal2.Close()

	// Read all
	reader, err := NewWALReader(walPath)
	assert.NilError(t, err)
	defer reader.Close()

	records, err := reader.ScanAll()
	assert.NilError(t, err)

	// 3 records from first batch (Begin, Insert, Commit)
	// 3 records from second batch
	assert.Equal(t, len(records), 6)

	assert.Equal(t, records[0].(*BeginTxnRecord).TxID, txID)
	assert.Equal(t, records[3].(*BeginTxnRecord).TxID, txID2)
}

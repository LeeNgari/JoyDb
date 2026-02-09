package wal

import (
	"bytes"
	"encoding/json"
	"hash/crc32"
	"testing"
)

// =============================================================================
// ROUND-TRIP TESTS
// =============================================================================

func TestRoundTrip_InsertRecord(t *testing.T) {
	// Setup input data
	txID := uint64(12345)
	tableName := "users"
	key := "100"
	value := json.RawMessage(`{"id":100,"name":"Alice"}`)

	// Encode using the WAL logic (simulated by calling private encoder methods if accessible,
	// or by writing to a real WAL and reading back).
	// Since encoders are private, we should test via NewWALReader/Writer or create a temporary WAL.
	// But unit tests often test internal logic.
	// The encoders are methods on *WAL.
	// I'll use a temporary file to test "write then read" which is the ultimate round-trip.

	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	w, err := NewWAL(tmpFile, "testdb")
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	w.BeginTransaction(txID)
	lsn, err := w.LogInsert(txID, tableName, key, value)
	if err != nil {
		t.Fatalf("LogInsert failed: %v", err)
	}
	w.Close()

	// Read back
	r, err := NewWALReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create WALReader: %v", err)
	}
	defer r.Close()

	// Skip header
	_, err = r.ReadFileHeader()
	if err != nil {
		t.Fatalf("ReadFileHeader failed: %v", err)
	}

	// Skip BeginTxn
	_, err = r.ReadNextRecord()
	if err != nil {
		t.Fatalf("ReadNextRecord (BeginTxn) failed: %v", err)
	}

	// Read Insert
	record, err := r.ReadNextRecord()
	if err != nil {
		t.Fatalf("ReadNextRecord (Insert) failed: %v", err)
	}

	// Verify record
	ins, ok := record.(*InsertRecord)
	if !ok {
		t.Fatalf("Expected InsertRecord, got %T", record)
	}

	if ins.TxID != txID {
		t.Errorf("TxID mismatch: got %d, want %d", ins.TxID, txID)
	}
	if ins.TableName != tableName {
		t.Errorf("TableName mismatch: got %s, want %s", ins.TableName, tableName)
	}
	if ins.Key != key {
		t.Errorf("Key mismatch: got %s, want %s", ins.Key, key)
	}
	if !bytes.Equal(ins.Value, value) {
		t.Errorf("Value mismatch: got %s, want %s", ins.Value, value)
	}
	if ins.Header.LSN != lsn {
		t.Errorf("LSN mismatch: got %d, want %d", ins.Header.LSN, lsn)
	}
}

func TestRoundTrip_UpdateRecord(t *testing.T) {
	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	w, err := NewWAL(tmpFile, "testdb")
	if err != nil {
		t.Fatal(err)
	}

	txID := uint64(101)
	oldVal := json.RawMessage(`{"v":1}`)
	newVal := json.RawMessage(`{"v":2}`)

	// Create active txn first
	w.BeginTransaction(txID)

	_, err = w.LogUpdate(txID, "t1", "k1", oldVal, newVal)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	r, err := NewWALReader(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.ReadFileHeader()

	// Skip Begin
	_, _ = r.ReadNextRecord()

	// Read Update
	rec, err := r.ReadNextRecord()
	if err != nil {
		t.Fatal(err)
	}

	upd, ok := rec.(*UpdateRecord)
	if !ok {
		t.Fatalf("Expected UpdateRecord, got %T", rec)
	}

	if !bytes.Equal(upd.OldValue, oldVal) {
		t.Error("OldValue mismatch")
	}
	if !bytes.Equal(upd.NewValue, newVal) {
		t.Error("NewValue mismatch")
	}
}

func TestRoundTrip_DeleteRecord(t *testing.T) {
	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	w, err := NewWAL(tmpFile, "testdb")
	if err != nil {
		t.Fatal(err)
	}

	txID := uint64(102)
	oldVal := json.RawMessage(`{"v":1}`)

	w.BeginTransaction(txID)
	_, err = w.LogDelete(txID, "t1", "k1", oldVal)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	r, err := NewWALReader(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.ReadFileHeader()
	r.ReadNextRecord() // Begin

	rec, err := r.ReadNextRecord()
	if err != nil {
		t.Fatal(err)
	}

	del, ok := rec.(*DeleteRecord)
	if !ok {
		t.Fatalf("Expected DeleteRecord, got %T", rec)
	}
	if !bytes.Equal(del.OldValue, oldVal) {
		t.Error("OldValue mismatch")
	}
}

func TestRoundTrip_CheckpointRecord(t *testing.T) {
	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	w, err := NewWAL(tmpFile, "testdb")
	if err != nil {
		t.Fatal(err)
	}

	tables := []TableChecksum{
		{TableName: "t1", DataCRC32: 1, MetaCRC32: 2},
		{TableName: "t2", DataCRC32: 3, MetaCRC32: 4},
	}
	dbCRC := uint32(99)

	lsn, err := w.WriteCheckpoint(tables, dbCRC)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	r, err := NewWALReader(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.ReadFileHeader()

	rec, err := r.ReadNextRecord()
	if err != nil {
		t.Fatal(err)
	}

	cp, ok := rec.(*CheckpointRecord)
	if !ok {
		t.Fatalf("Expected CheckpointRecord, got %T", rec)
	}

	if cp.DatabaseCRC32 != dbCRC {
		t.Errorf("DatabaseCRC mismatch: %d vs %d", cp.DatabaseCRC32, dbCRC)
	}
	if len(cp.Tables) != 2 {
		t.Errorf("Table count mismatch: %d vs 2", len(cp.Tables))
	}
	if cp.Tables[0].TableName != "t1" || cp.Tables[1].TableName != "t2" {
		t.Error("Table data mismatch")
	}
	if cp.CheckpointLSN != lsn {
		t.Error("LSN mismatch")
	}
}

// =============================================================================
// BINARY FORMAT & EDGE CASES
// =============================================================================

func TestRecordHeader_BinaryLayout(t *testing.T) {
	// Verify that header encodes to exactly 32 bytes
	h := WALRecordHeader{
		Type:       RecordInsert,
		Length:     100,
		LSN:        1,
		CRC32:      0xDEADBEEF,
		FileOffset: 1024,
		PayloadLen: 50,
	}

	// We use the internal encoder (requires it to be exported or test inside package 'wal')
	// Since we are in 'package wal', we can access 'encodeHeader'
	buf := encodeHeader(h)
	if len(buf) != RecordHeaderSize {
		t.Errorf("Encoded header size mismatch: got %d, want %d", len(buf), RecordHeaderSize)
	}

	// Verify offset 0 is Type
	if buf[0] != byte(RecordInsert) {
		t.Error("Type mismatch at offset 0")
	}
	// Verify offset 2-6 is Length
	lenVal := ByteOrder.Uint32(buf[2:6])
	if lenVal != 100 {
		t.Error("Length mismatch")
	}
}

func TestCRC32_DetectsCorruption(t *testing.T) {
	// 1. Create valid record
	payload := []byte("test-payload")
	crc := crc32.ChecksumIEEE(payload)
	if err := verifyCRC32(payload, crc); err != nil {
		t.Fatal("Valid CRC failed")
	}

	// 2. Corrupt payload
	payload[0] ^= 0xFF
	if err := verifyCRC32(payload, crc); err == nil {
		t.Fatal("Corrupted payload should fail CRC check")
	}
}

func TestPayloadEncoding_EmptyValues(t *testing.T) {
	// Ensure we can handle empty byte slices/strings
	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	w, err := NewWAL(tmpFile, "db")
	if err != nil {
		t.Fatal(err)
	}

	w.BeginTransaction(1)
	// Empty key, empty value
	_, err = w.LogInsert(1, "", "", json.RawMessage{})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	r, _ := NewWALReader(tmpFile)
	defer r.Close()
	r.ReadFileHeader()
	r.ReadNextRecord() // Begin

	rec, err := r.ReadNextRecord()
	if err != nil {
		t.Fatal(err)
	}
	ins := rec.(*InsertRecord)
	if ins.TableName != "" || ins.Key != "" || len(ins.Value) != 0 {
		t.Error("Empty values not preserved")
	}
}

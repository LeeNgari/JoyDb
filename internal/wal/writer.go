package wal

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"time"
)

// ===========================================================================
// WAL WRITER OPERATIONS
// ===========================================================================
//
// All write operations follow this pattern:
// 1. Acquire mutex
// 2. Allocate LSN
// 3. Encode payload
// 4. Calculate CRC32
// 5. Build header with length and offset
// 6. Write header + payload + padding
// 7. Update currentOffset
// 8. Release mutex
//
// Sync (fsync) is NOT called on every write for performance.
// Call Sync() explicitly or use Commit() for durability guarantee.
//
// ===========================================================================

// BeginTransaction writes a BeginTxn record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) BeginTransaction(txID uint64) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Encode payload: TxID (8 bytes)
	payload := make([]byte, 8)
	ByteOrder.PutUint64(payload, txID)

	// Write record
	lsn, err := w.writeRecord(RecordBeginTxn, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write BeginTxn record: %w", err)
	}

	// Track active transaction
	w.activeTxns[txID] = &TxnState{
		ID:    txID,
		State: TxnActive,
	}

	return lsn, nil
}

// LogInsert writes an Insert record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) LogInsert(txID uint64, tableName string, key string, value json.RawMessage) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID + TableName + Key + Value
	payload := w.encodeInsertPayload(txID, tableName, key, value)

	// Write record
	lsn, err := w.writeRecord(RecordInsert, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Insert record: %w", err)
	}

	return lsn, nil
}

// LogUpdate writes an Update record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) LogUpdate(txID uint64, tableName string, key string, oldValue json.RawMessage, newValue json.RawMessage) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID + TableName + Key + OldValue + NewValue
	payload := w.encodeUpdatePayload(txID, tableName, key, oldValue, newValue)

	// Write record
	lsn, err := w.writeRecord(RecordUpdate, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Update record: %w", err)
	}

	return lsn, nil
}

// LogDelete writes a Delete record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) LogDelete(txID uint64, tableName string, key string, oldValue json.RawMessage) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID + TableName + Key + OldValue
	payload := w.encodeDeletePayload(txID, tableName, key, oldValue)

	// Write record
	lsn, err := w.writeRecord(RecordDelete, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Delete record: %w", err)
	}

	return lsn, nil
}

// Commit writes a Commit record to the WAL and fsyncs
// This makes the transaction durable
// Returns the LSN assigned to this record
func (w *WAL) Commit(txID uint64) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID (8 bytes)
	payload := make([]byte, 8)
	ByteOrder.PutUint64(payload, txID)

	// Write record
	lsn, err := w.writeRecord(RecordCommit, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Commit record: %w", err)
	}

	// Fsync to ensure durability
	if err := w.file.Sync(); err != nil {
		return 0, fmt.Errorf("failed to fsync after commit: %w", err)
	}

	// Update flushed LSN
	w.flushedLSN = lsn

	// Update transaction state and remove from active
	w.activeTxns[txID].State = TxnCommitted
	delete(w.activeTxns, txID)

	return lsn, nil
}

// Abort writes an Abort record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) Abort(txID uint64) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID (8 bytes)
	payload := make([]byte, 8)
	ByteOrder.PutUint64(payload, txID)

	// Write record
	lsn, err := w.writeRecord(RecordAbort, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Abort record: %w", err)
	}

	// Update transaction state and remove from active
	w.activeTxns[txID].State = TxnAborted
	delete(w.activeTxns, txID)

	return lsn, nil
}

// WriteCheckpoint writes a Checkpoint record to the WAL
// This should be called after successfully persisting all dirty tables to JSON
// Returns the LSN assigned to this record
func (w *WAL) WriteCheckpoint(tables []TableChecksum, databaseCRC32 uint32) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Build checkpoint payload
	payload := w.encodeCheckpointPayload(tables, databaseCRC32)

	// Write record
	lsn, err := w.writeRecord(RecordCheckpoint, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Checkpoint record: %w", err)
	}

	// Fsync to ensure checkpoint is durable
	if err := w.file.Sync(); err != nil {
		return 0, fmt.Errorf("failed to fsync after checkpoint: %w", err)
	}

	// Update flushed LSN and checkpoint LSN
	w.flushedLSN = lsn
	w.lastCheckpoint = lsn

	return lsn, nil
}

// ===========================================================================
// INTERNAL HELPERS
// ===========================================================================

// verifyActiveTxn checks if a transaction is active
// Must be called with mutex held
func (w *WAL) verifyActiveTxn(txID uint64) error {
	txn, exists := w.activeTxns[txID]
	if !exists {
		return fmt.Errorf("transaction %d not found", txID)
	}
	if txn.State != TxnActive {
		return fmt.Errorf("transaction %d is not active (state: %s)", txID, txn.State)
	}
	return nil
}

// writeRecord writes a complete WAL record (header + payload + padding)
// Must be called with mutex held
func (w *WAL) writeRecord(recordType RecordType, payload []byte) (uint64, error) {
	// Allocate LSN
	lsn := w.allocateLSN()

	// Calculate CRC32 of payload
	crc := crc32.ChecksumIEEE(payload)

	// Calculate total length with alignment
	payloadLen := len(payload)
	totalLen := RecordHeaderSize + payloadLen
	alignedLen := AlignTo8(totalLen)
	paddingLen := alignedLen - totalLen

	// Build header
	header := WALRecordHeader{
		Type:       recordType,
		Length:     uint32(alignedLen),
		LSN:        lsn,
		CRC32:      crc,
		FileOffset: uint32(w.currentOffset),
	}

	// Encode header
	headerBytes := encodeHeader(header)

	// Write header
	if _, err := w.file.Write(headerBytes); err != nil {
		return 0, fmt.Errorf("failed to write header: %w", err)
	}

	// Write payload
	if _, err := w.file.Write(payload); err != nil {
		return 0, fmt.Errorf("failed to write payload: %w", err)
	}

	// Write padding if needed
	if paddingLen > 0 {
		padding := make([]byte, paddingLen)
		if _, err := w.file.Write(padding); err != nil {
			return 0, fmt.Errorf("failed to write padding: %w", err)
		}
	}

	// Update current offset
	w.currentOffset += uint64(alignedLen)

	return lsn, nil
}

// encodeHeader encodes a WALRecordHeader to bytes (24 bytes)
func encodeHeader(h WALRecordHeader) []byte {
	buf := make([]byte, RecordHeaderSize)

	// Type (1 byte)
	buf[0] = byte(h.Type)

	// Padding (1 byte) - already zero

	// Length (4 bytes) at offset 2
	ByteOrder.PutUint32(buf[2:6], h.Length)

	// LSN (8 bytes) at offset 6
	ByteOrder.PutUint64(buf[6:14], h.LSN)

	// CRC32 (4 bytes) at offset 14
	ByteOrder.PutUint32(buf[14:18], h.CRC32)

	// FileOffset (4 bytes) at offset 18
	ByteOrder.PutUint32(buf[18:22], h.FileOffset)

	// Remaining 2 bytes are padding (already zero)

	return buf
}

// ===========================================================================
// PAYLOAD ENCODERS
// ===========================================================================

// encodeInsertPayload encodes the payload for an Insert record
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + ValueLen(4) + Value
func (w *WAL) encodeInsertPayload(txID uint64, tableName string, key string, value json.RawMessage) []byte {
	// Calculate total size
	size := 8 + 2 + len(tableName) + 2 + len(key) + 4 + len(value)
	buf := make([]byte, size)
	offset := 0

	// TxID (8 bytes)
	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	// TableName with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)
	offset += len(tableName)

	// Key with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(key)))
	offset += 2
	copy(buf[offset:], key)
	offset += len(key)

	// Value with length prefix (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(value)))
	offset += 4
	copy(buf[offset:], value)

	return buf
}

// encodeUpdatePayload encodes the payload for an Update record
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + OldValueLen(4) + OldValue + NewValueLen(4) + NewValue
func (w *WAL) encodeUpdatePayload(txID uint64, tableName string, key string, oldValue json.RawMessage, newValue json.RawMessage) []byte {
	// Calculate total size
	size := 8 + 2 + len(tableName) + 2 + len(key) + 4 + len(oldValue) + 4 + len(newValue)
	buf := make([]byte, size)
	offset := 0

	// TxID (8 bytes)
	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	// TableName with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)
	offset += len(tableName)

	// Key with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(key)))
	offset += 2
	copy(buf[offset:], key)
	offset += len(key)

	// OldValue with length prefix (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(oldValue)))
	offset += 4
	copy(buf[offset:], oldValue)
	offset += len(oldValue)

	// NewValue with length prefix (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(newValue)))
	offset += 4
	copy(buf[offset:], newValue)

	return buf
}

// encodeDeletePayload encodes the payload for a Delete record
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + OldValueLen(4) + OldValue
func (w *WAL) encodeDeletePayload(txID uint64, tableName string, key string, oldValue json.RawMessage) []byte {
	// Calculate total size
	size := 8 + 2 + len(tableName) + 2 + len(key) + 4 + len(oldValue)
	buf := make([]byte, size)
	offset := 0

	// TxID (8 bytes)
	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	// TableName with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)
	offset += len(tableName)

	// Key with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(key)))
	offset += 2
	copy(buf[offset:], key)
	offset += len(key)

	// OldValue with length prefix (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(oldValue)))
	offset += 4
	copy(buf[offset:], oldValue)

	return buf
}

// encodeCheckpointPayload encodes the payload for a Checkpoint record
// Format: CheckpointLSN(8) + CheckpointOffset(8) + LastFlushedLSN(8) + Timestamp(8) +
//
//	DatabaseCRC32(4) + TableCount(4) + [TableChecksums...]
//
// Each TableChecksum: TableNameLen(2) + TableName + DataCRC32(4) + MetaCRC32(4)
func (w *WAL) encodeCheckpointPayload(tables []TableChecksum, databaseCRC32 uint32) []byte {
	// Calculate size for tables
	tablesSize := 0
	for _, t := range tables {
		tablesSize += 2 + len(t.TableName) + 4 + 4
	}

	// Total size: fixed fields + tables
	size := 8 + 8 + 8 + 8 + 4 + 4 + tablesSize
	buf := make([]byte, size)
	offset := 0

	// CheckpointLSN (8 bytes) - will be set to current LSN
	ByteOrder.PutUint64(buf[offset:], w.nextLSN)
	offset += 8

	// CheckpointOffset (8 bytes) - current file offset
	ByteOrder.PutUint64(buf[offset:], w.currentOffset)
	offset += 8

	// LastFlushedLSN (8 bytes)
	ByteOrder.PutUint64(buf[offset:], w.flushedLSN)
	offset += 8

	// Timestamp (8 bytes)
	ByteOrder.PutUint64(buf[offset:], uint64(time.Now().Unix()))
	offset += 8

	// DatabaseCRC32 (4 bytes)
	ByteOrder.PutUint32(buf[offset:], databaseCRC32)
	offset += 4

	// TableCount (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(tables)))
	offset += 4

	// Tables
	for _, t := range tables {
		// TableName with length prefix (2 bytes)
		ByteOrder.PutUint16(buf[offset:], uint16(len(t.TableName)))
		offset += 2
		copy(buf[offset:], t.TableName)
		offset += len(t.TableName)

		// DataCRC32 (4 bytes)
		ByteOrder.PutUint32(buf[offset:], t.DataCRC32)
		offset += 4

		// MetaCRC32 (4 bytes)
		ByteOrder.PutUint32(buf[offset:], t.MetaCRC32)
		offset += 4
	}

	return buf
}
